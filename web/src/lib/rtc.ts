// Define all kinds of messages that I can receive/send from/to the server
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

// [TODO] Revisit the ICE servers if too many peers fail
const ICE_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.l.google.com:19302" },
  {
    urls: [
      "turn:openrelay.metered.ca:80",
      "turn:openrelay.metered.ca:443",
      "turn:openrelay.metered.ca:443?transport=tcp",
    ],
    username: "openrelayproject",
    credential: "openrelayproject",
  },
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
    await this.sendAnswer(from, pc);
  }

  private async handleAnswer(from: string, sdp: RTCSessionDescriptionInit) {
    const pc = this.connections.get(from);
    if (!pc) return;
    await pc.setRemoteDescription(sdp);
  }

  private async handleIce(from: string, candidate: RTCIceCandidateInit) {
    const pc = this.connections.get(from);
    if (!pc) return;
    try {
      await pc.addIceCandidate(candidate);
    } catch {}
  }

  private dropPeer(peerId: string) {
    this.connections.get(peerId)?.close();
    this.connections.delete(peerId);
    this.channels.delete(peerId);
    this.handlers.onPeersChanged?.([...this.channels.keys()]);
  }
}
