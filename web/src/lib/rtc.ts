// Define all kinds of messages that I can receive/send from/to the server

import { Deferred } from "./deferred";

// [TODO] how to keep these in accordance with the Go server?
enum SignalType {
  Welcome = "welcome",
  PeerJoined = "peer-joined",
  PeerLeft = "peer-left",
  Offer = "offer",
  Answer = "answer",
  Ice = "ice-candidate",
}
type Welcome = {
  type: SignalType.Welcome;
  id: string;
  peers: string[];
};
type PeerJoined = {
  type: SignalType.PeerJoined;
  id: string;
};
type PeerLeft = {
  type: SignalType.PeerLeft;
  id: string;
};
type Offer = {
  type: SignalType.Offer;
  from: string;
  sdp: RTCSessionDescriptionInit;
};
type Answer = {
  type: SignalType.Answer;
  from: string;
  sdp: RTCSessionDescriptionInit;
};
type Ice = {
  type: SignalType.Ice;
  from: string;
  candidate: RTCIceCandidateInit;
};
type SignalMessage = Welcome | PeerJoined | PeerLeft | Offer | Answer | Ice;

type OutgoingOffer = {
  type: "offer";
  to: string;
  sdp: RTCSessionDescriptionInit;
};
type OutgoingAnswer = {
  type: "answer";
  to: string;
  sdp: RTCSessionDescriptionInit;
};
type OutgoingIce = {
  type: "ice-candidate";
  to: string;
  candidate: RTCIceCandidateInit;
};
type OutgoingSignal = OutgoingOffer | OutgoingAnswer | OutgoingIce;

const TURN_USERNAME = import.meta.env.VITE_TURN_USERNAME as string | undefined;
const TURN_CREDENTIAL = import.meta.env.VITE_TURN_CREDENTIAL as
  | string
  | undefined;

console.log(TURN_USERNAME, TURN_CREDENTIAL);

const ICE_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.cloudflare.com:3478" },
  ...(TURN_USERNAME && TURN_CREDENTIAL
    ? [
        {
          urls: [
            "turn:turn.cloudflare.com:3478?transport=udp",
            "turn:turn.cloudflare.com:3478?transport=tcp",
            "turns:turn.cloudflare.com:5349?transport=tcp",
          ],
          username: TURN_USERNAME,
          credential: TURN_CREDENTIAL,
        },
      ]
    : []),
];
const CHANNEL_LABEL = "yata";

type Handlers = {
  onData?: (peerId: string, data: string) => void; // Receive updates to the document
  onPeersChanged?: (peers: string[]) => void; // To maybe update the UI
};

export class RTC {
  private myId: string | null = null;
  private connections = new Map<string, RTCPeerConnection>();
  private channels = new Map<string, RTCDataChannel>();
  private handlers: Handlers;
  private sendRaw: (raw: string) => void = () => {};
  private pendingIce = new Map<string, Deferred<void>[]>();

  constructor(handlers: Handlers = {}) {
    this.handlers = handlers;
  }

  public setSendMethod(sendRaw: (raw: string) => void) {
    this.sendRaw = sendRaw;
  }

  public handleSignal(raw: string) {
    let msg: SignalMessage;
    try {
      msg = JSON.parse(raw);
    } catch {
      // Invalid JSON message
      return;
    }

    console.log("Received signal", msg);
    switch (msg.type) {
      case SignalType.Welcome:
        this.myId = msg.id;
        msg.peers.forEach((id) => this.initiate(id));
        break;
      case SignalType.PeerJoined:
        // New peer initiates; we just wait for their offer.
        break;
      case SignalType.PeerLeft:
        this.dropPeer(msg.id);
        break;
      case SignalType.Offer:
        this.handleOffer(msg.from, msg.sdp);
        break;
      case SignalType.Answer:
        this.handleAnswer(msg.from, msg.sdp);
        break;
      case SignalType.Ice:
        this.handleIce(msg.from, msg.candidate);
        break;
    }
  }

  public sendTo(peerId: string, data: string) {
    const ch = this.channels.get(peerId);
    if (ch?.readyState === "open") ch.send(data);
  }

  /**
   * Broadcasts a message to all connected peers
   * @param data Data to broadcast
   */
  public broadcast(data: string) {
    console.log("broadcast", this.channels.size, data);
    this.channels.forEach((ch) => {
      console.log("  ->", ch.readyState);
      if (ch.readyState === "open") ch.send(data);
    });
  }

  public close() {
    this.connections.forEach((pc) => pc.close());
    this.connections.clear();
    this.channels.clear();
  }

  get id(): string | null {
    return this.myId;
  }

  private send(msg: OutgoingSignal) {
    this.sendRaw(JSON.stringify(msg));
  }

  private async sendOffer(to: string, pc: RTCPeerConnection) {
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    this.send({ type: "offer", to, sdp: offer });
  }

  private async sendAnswer(to: string, pc: RTCPeerConnection) {
    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);
    this.send({ type: "answer", to, sdp: answer });
  }

  private sendIce(to: string, candidate: RTCIceCandidateInit) {
    this.send({ type: "ice-candidate", to, candidate });
  }

  private createPeerConnection(peerId: string): RTCPeerConnection {
    const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
    pc.onicecandidate = (e) => {
      if (e.candidate) {
        this.sendIce(peerId, e.candidate.toJSON());
        console.log("Sent ICE candidate for peer", peerId);
      }
    };
    pc.ondatachannel = (e) => this.attachChannel(peerId, e.channel);
    pc.onconnectionstatechange = () => {
      console.log(
        "Peer connection state changed for peer",
        peerId,
        pc.connectionState,
      );
      if (pc.connectionState === "failed" || pc.connectionState === "closed") {
        this.dropPeer(peerId);
      }
    };
    this.connections.set(peerId, pc);
    this.pendingIce.set(peerId, []);
    console.log("Created peer connection for peer", peerId);
    return pc;
  }

  private attachChannel(peerId: string, ch: RTCDataChannel) {
    this.channels.set(peerId, ch);
    ch.onmessage = (e) => this.handlers.onData?.(peerId, e.data);
    ch.onopen = () => this.handlers.onPeersChanged?.([...this.channels.keys()]);
    ch.onclose = () => {
      this.channels.delete(peerId);
      this.handlers.onPeersChanged?.([...this.channels.keys()]);
    };
  }

  private async initiate(peerId: string) {
    const pc = this.createPeerConnection(peerId);
    const ch = pc.createDataChannel(CHANNEL_LABEL);
    this.attachChannel(peerId, ch);
    await this.sendOffer(peerId, pc);
  }

  private async handleOffer(from: string, sdp: RTCSessionDescriptionInit) {
    const pc = this.createPeerConnection(from);
    await pc.setRemoteDescription(sdp);
    this.flushPendingIce(from);
    await this.sendAnswer(from, pc);
  }

  /**
   * Resolves any pending ICE candidates for the given peer (in case
   * candidates arrived before description).
   * This is called when we receive an answer from a peer, which means
   * we can now add their ICE candidates to our connection.
   * @param peerId The peer id to flush pending ICE candidates for.
   */
  private flushPendingIce(peerId: string) {
    const queuedIced = this.pendingIce.get(peerId) ?? [];
    queuedIced.forEach((candidateDeferred) => {
      candidateDeferred.resolve();
    });
    this.pendingIce.delete(peerId);
  }

  private async handleAnswer(from: string, sdp: RTCSessionDescriptionInit) {
    const pc = this.connections.get(from);
    if (!pc) return;
    await pc.setRemoteDescription(sdp);

    this.flushPendingIce(from);
  }

  private async handleIce(from: string, candidate: RTCIceCandidateInit) {
    const pc = this.connections.get(from);
    const waiting = this.pendingIce.get(from);

    if (!pc) {
      console.log("No peer connection for", from);
      return;
    }

    if (waiting && !pc.remoteDescription) {
      const deferred = new Deferred<void>();
      waiting.push(deferred);
      await deferred.promise;
    }

    try {
      await pc.addIceCandidate(candidate);
    } catch (e) {
      console.error("Failed to add ICE candidate for peer", from, e);
    }
  }

  private dropPeer(peerId: string) {
    this.connections.get(peerId)?.close();
    this.connections.delete(peerId);
    this.channels.delete(peerId);
    this.handlers.onPeersChanged?.([...this.channels.keys()]);
  }
}
