import { YataText } from "../../../crdt/text";
import { UUID, YataItem, YataStateVector } from "../../../crdt/types";
import { RTC } from "./rtc";

type WireStateVector = Record<string, number>;

type WireMessage =
  | { kind: "stateVector"; sv: WireStateVector }
  | { kind: "update"; items: YataItem[]; deletes: string[] };

export class TextPeer {
  private readonly text: YataText;
  // tombstone log in received order
  private readonly tombstoneOrder: string[];
  private readonly seenTombstones: Set<string>;

  /**
   * snapshot of state at the last broadcastChanges call
   * or the most recently applied incoming update
   */
  private lastBroadcastSV: YataStateVector;

  /**
   * number of tombstones broadcasted at the last broadcastChanges call
   * or the most recently applied incoming update
   */
  private lastBroadcastTombstoneCount: number;

  constructor(uuid: UUID) {
    this.text = new YataText(uuid);
    this.tombstoneOrder = [];
    this.seenTombstones = new Set();
    this.lastBroadcastSV = new YataStateVector();
    this.lastBroadcastTombstoneCount = 0;
  }

  /**
  Inserts a character at the given index. The new struct is not sent
  on the wire here; broadcastChanges (called on the periodic timer)
  ships it together with any other edits batched in the same window.
  @param index - The index at which to insert the character.
  @param character - The character to insert.
  @requires character is a single-character string
  @throws Error if index is out of range (must be in [0, this.length()])
  **/
  insert(index: number, character: string): void {
    this.text.insertItem(index, character);
  }

  /**
  Deletes the character at the given index. The new tombstone is not
  sent on the wire here; broadcastChanges ships it on the next tick.
  @param index - The index of the character to delete.
  @throws Error if index is out of range (must be in [0, this.length()))
  **/
  delete(index: number): void {
    this.text.deleteCharacter(index);
    this.refreshTombstoneOrder();
  }

  /**
  Sends our state vector to a peer to kick off a one-shot catch-up.
  Only used on initial connect / reconnect; ongoing edits propagate
  via broadcastChanges instead.
  @param rtc instance to send through.
  @param peerId the peer to sync with.
  **/
  syncWithPeer(rtc: RTC, peerId: string): void {
    const msg: WireMessage = {
      kind: "stateVector",
      sv: this.text.getStateVector().getWire(),
    };
    rtc.sendTo(peerId, JSON.stringify(msg));
    console.log("syncWithPeer", peerId, JSON.stringify(msg));
  }

  /**
  Broadcasts operations since last broadcast.
  @param rtc - The RTC instance to broadcast through.
  **/
  broadcastChanges(rtc: RTC): void {
    const items = this.text.makeUpdate(this.lastBroadcastSV);
    const deletes = this.tombstoneOrder.slice(
      this.lastBroadcastTombstoneCount,
    );
    if (items.length === 0 && deletes.length === 0) return;
    const msg: WireMessage = { kind: "update", items, deletes };
    rtc.broadcast(JSON.stringify(msg));
    this.lastBroadcastSV = this.text.getStateVector();
    this.lastBroadcastTombstoneCount = this.tombstoneOrder.length;
    console.log("broadcastChanges", JSON.stringify(msg));
  }

  /**
  Routes an incoming message from a peer through the sync protocol.
  - On "stateVector" (only fired on a peer's join handshake): reply with the
    insert diff plus the full tombstone set.
  - On "update": apply the items and tombstones, refresh our local tombstone log
    and advance last broadcast cursor to avoid broadcasting the same state.
  @param rtc - The RTC instance to send through.
  @param peerId - The peer the message came from.
  @param raw - The raw JSON-encoded WireMessage.
  **/
  handleMessage(rtc: RTC, peerId: string, raw: string): void {
    let msg: WireMessage;
    try {
      msg = JSON.parse(raw) as WireMessage;
    } catch {
      return;
    }

    if (msg.kind === "stateVector") {
      const reply: WireMessage = {
        kind: "update",
        items: this.text.makeUpdate(YataStateVector.fromWire(msg.sv)),
        deletes: this.text.getTombstoneKeys(),
      };
      if (reply.items.length === 0 && reply.deletes.length === 0) return;
      rtc.sendTo(peerId, JSON.stringify(reply));
      console.log("syncReply", peerId, JSON.stringify(reply));
      return;
    }

    if (msg.kind === "update") {
      this.text.applyUpdate(msg.items);
      this.text.applyTombstones(msg.deletes);
      this.refreshTombstoneOrder();
      this.lastBroadcastSV = this.text.getStateVector();
      this.lastBroadcastTombstoneCount = this.tombstoneOrder.length;
      console.log("applyUpdate", peerId, JSON.stringify(msg));
      return;
    }
  }

  /**
  Pull any tombstones the CRDT now knows about that we haven't seen yet,
  in the CRDT's reported order, and append them to the tombstone log.
  **/
  private refreshTombstoneOrder(): void {
    for (const key of this.text.getTombstoneKeys()) {
      if (this.seenTombstones.has(key)) continue;
      this.seenTombstones.add(key);
      this.tombstoneOrder.push(key);
    }
  }

  /**
  Returns the text as a string.
  @returns the text as a string.
  **/
  toString(): string {
    return this.text.toString();
  }
}
