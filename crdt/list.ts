import { ID, UUID, YataItem, YataStateVector, YataType } from "./types";

export class YataList implements YataType {
  private readonly owner: ID;
  private readonly itemById: Map<ID, YataItem>;
  private readonly items: YataItem[];
  private stateVector: YataStateVector;

  constructor(uuid: UUID) {
    this.owner = { userID: uuid, opCounter: 0 };
    this.items = new Array<YataItem>(2); // TODO: edit sentinels
    this.itemById = new Map<ID, YataItem>();
    this.stateVector = new Map<UUID, number>();
    this.stateVector.set(uuid, 0);
  }

  getStateVector(): YataStateVector {
    throw new Error("not implemented");
  }

  private integrate(item: YataItem): void {
    throw new Error("not implemented");
  }

  applyUpdate(update: Uint8Array): void {
    throw new Error("not implemented");
  }

  encodeStateVector(): Uint8Array {
    throw new Error("not implemented");
  }

  encodeStateAsUpdate(encodedTargetStateVector?: Uint8Array): Uint8Array {
    throw new Error("not implemented");
  }

  /**
  Inserts an item at the given index. 
  If the index is greater than the length of the list, the item is appended to the end of the list.
  @param index - The index at which to insert the item.
  @requires index is a non-negative integer
  @requires content is not null
  **/
  insertItem(index: number, content: unknown): void {
    throw new Error("not implemented");
  }

  /**
  Marks the item at the given index as deleted. The item is not removed from the list.
  @param index - The index of the item to mark as deleted.
  @requires index is a non-negative integer and index < this.length()
  **/
  deleteItem(index: number): void {
    throw new Error("not implemented");
  }

  /**
  Length of the list.
  @returns the length of the list.
  **/
  length(): number {
    throw new Error("not implemented");
  }

  /**
  Returns the items in the list.
  @returns an array of the items in the list.
  **/
  getItems(): unknown[] {
    throw new Error("not implemented");
  }
}
