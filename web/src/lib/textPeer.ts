import { YataText } from "../../../crdt/text";
import { UUID, YataItem, YataStateVector } from "../../../crdt/types";
import { RTC } from "./rtc";

type WireStateVector = Record<string, number>;

type WireMessage =
  | { kind: "stateVector"; sv: WireStateVector }
  | { kind: "update"; items: YataItem[]; deletes: string[] };

export class TextPeer {
  private readonly text: YataText;

  constructor(uuid: UUID) {
    this.text = new YataText(uuid);
  }

  /**
  Inserts a character at the given index.
  @param index - The index at which to insert the character.
  @param character - The character to insert.
  @requires character is a single-character string
  @throws Error if index is out of range (must be in [0, this.length()])
  **/
  insert(index: number, character: string): void {
    this.text.insertItem(index, character);
  }

  /**
  Deletes the character at the given index.
  @param index - The index of the character to delete.
  @throws Error if index is out of range (must be in [0, this.length()))
  **/
  delete(index: number): void {
    this.text.deleteCharacter(index);
  }

  /**
  Sends our state vector to a peer
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
  Routes an incoming message from a peer through the sync protocol.
  - On "stateVector": reply with the SV-filtered insert diff plus all
    currently-tombstoned item keys.
  - On "update": apply the items via the CRDT update path, then layer
    the tombstones on top.
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
      console.log("update", peerId, JSON.stringify(reply));
      return;
    }

    if (msg.kind === "update") {
      this.text.applyUpdate(msg.items);
      this.text.applyTombstones(msg.deletes);
      console.log("applyUpdate", peerId, JSON.stringify(msg));
      return;
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
