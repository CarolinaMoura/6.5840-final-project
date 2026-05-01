import { ID, UUID, YataItem, YataStateVector, YataType } from "./types";

export class YataList implements YataType {
  owner: ID;
  items: YataItem[];
  stateVector: YataStateVector;

  constructor(uuid: UUID) {
    this.owner = { userID: uuid, opCounter: 0 };
    this.items = new Array<YataItem>(2);
    this.stateVector = [this.owner];
  }

  getStateVector(): YataStateVector {
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

  insertItem(index: number, content: unknown): void {
    throw new Error("not implemented");
  }

  deleteItem(index: number): void {
    throw new Error("not implemented");
  }

  length(): number {
    throw new Error("not implemented");
  }

  getItems(): unknown[] {
    throw new Error("not implemented");
  }
}
