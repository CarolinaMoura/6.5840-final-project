import { YataList } from "./list";
import { UUID, YataItem, YataStateVector, YataType } from "./types";

export class YataText implements YataType {
  private readonly list: YataList;

  constructor(uuid: UUID) {
    this.list = new YataList(uuid);
  }

  getStateVector(): YataStateVector {
    return this.list.getStateVector();
  }

  applyUpdate(items: YataItem[]): void {
    this.list.applyUpdate(items);
  }

  makeUpdate(targetStateVector?: YataStateVector): YataItem[] {
    return this.list.makeUpdate(targetStateVector);
  }

  /**
  Inserts a character at the given index.
  @param index - The index at which to insert the character.
  @param character - The character to insert.
  @requires character is a single-character string
  @throws Error if index is out of range (must be in [0, this.length()])
  **/
  insertItem(index: number, character: string): void {
    const len = this.list.length();
    if (!Number.isInteger(index) || index < 0 || index > len) {
      throw new Error(
        `YataText.insertItem: index ${index} out of range (length ${len})`
      );
    }
    this.list.insertItem(index, character);
  }

  /**
  Deletes the character at the given index.
  @param index - The index of the character to delete.
  @throws Error if index is out of range (must be in [0, this.length()))
  **/
  deleteCharacter(index: number): void {
    const len = this.list.length();
    if (!Number.isInteger(index) || index < 0 || index >= len) {
      throw new Error(
        `YataText.deleteCharacter: index ${index} out of range (length ${len})`
      );
    }
    this.list.deleteItem(index);
  }

  /**
  Length of the text.
  @returns the number of characters in the text.
  **/
  length(): number {
    return this.list.length();
  }

  /**
  Returns the keys of every currently-tombstoned non-sentinel item.
  @returns an array of ID.key strings.
  **/
  getTombstoneKeys(): string[] {
    return this.list.getTombstoneKeys();
  }

  /**
  Marks each item whose key is in `keys` as deleted. Unknown keys are ignored.
  @param keys - the keys (as produced by ID.key) of items to tombstone.
  **/
  applyTombstones(keys: string[]): void {
    this.list.applyTombstones(keys);
  }

  /**
  Returns the text.
  @returns the text as a string.
  **/
  toString(): string {
    return this.list.getItems().map((c) => String(c)).join("");
  }
}
