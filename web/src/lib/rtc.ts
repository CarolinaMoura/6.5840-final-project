// Define all kinds of messages that I can receive/send from/to the server
enum SignalType {
  Welcome,
  PeerJoined,
  PeerLeft,
  Offer,
  Answer,
  Ice,
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

// Revisit the ICE servers if too many peers fail
const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];
const CHANNEL_LABEL = "yata";

type Handlers = {
  onData?: (peerId: string, data: string) => void; // Receive updates to the document
  onPeersChanged?: (peers: string[]) => void; // To maybe update the UI
};

export class RTC {
  private myId: string | null = null;
  private connections = new Map<string, RTCPeerConnection>();
  private channels = new Map<string, RTCDataChannel>();
  private sendRaw: (raw: string) => void;
  private handlers: Handlers;

  /**
   * Creates a new peer-to-peer connection manager.
   *
   * @param sendRaw Function to send a raw message to the server.
   * @param handlers Optional handlers for data and peer changes
   */
  constructor(sendRaw: (raw: string) => void, handlers: Handlers = {}) {
    this.sendRaw = sendRaw;
    this.handlers = handlers;
  }

  public handleSignal(raw: string) {
    let msg: SignalMessage;
    try {
      msg = JSON.parse(raw);
    } catch {
      // Invalid JSON message
      return;
    }

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
    this.channels.forEach((ch) => {
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

  private send(msg: object) {
    this.sendRaw(JSON.stringify(msg));
  }

  private createPeerConnection(peerId: string): RTCPeerConnection {
    const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
    pc.onicecandidate = (e) => {
      if (e.candidate) {
        this.send({
          type: "ice-candidate",
          to: peerId,
          candidate: e.candidate.toJSON(),
        });
      }
    };
    pc.ondatachannel = (e) => this.attachChannel(peerId, e.channel);
    pc.onconnectionstatechange = () => {
      if (pc.connectionState === "failed" || pc.connectionState === "closed") {
        this.dropPeer(peerId);
      }
    };
    this.connections.set(peerId, pc);
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
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    this.send({ type: "offer", to: peerId, sdp: offer });
  }

  private async handleOffer(from: string, sdp: RTCSessionDescriptionInit) {
    const pc = this.createPeerConnection(from);
    await pc.setRemoteDescription(sdp);
    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);
    this.send({ type: "answer", to: from, sdp: answer });
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
