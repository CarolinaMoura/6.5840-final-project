import { YataText } from "../../../crdt/text";
import { UUID } from "../../../crdt/types";
import { RTC } from "./rtc";

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
  Broadcasts the current state of the text to all connected peers.
  @param rtc - The RTC instance to broadcast through.
  **/
  broadcast(rtc: RTC): void {
    const update = this.text.encodeStateAsUpdate();
    rtc.broadcast(new TextDecoder().decode(update));
  }

  /**
  Applies a remote update received from a peer.
  @param update - The serialized update payload as delivered over the wire.
  **/
  applyUpdate(update: string): void {
    this.text.applyUpdate(new TextEncoder().encode(update));
  }

  /**
  Returns the text as a string.
  @returns the text as a string.
  **/
  toString(): string {
    return this.text.toString()
  }
}
