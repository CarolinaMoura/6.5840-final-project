export type UUID = number;

export class ID {
  constructor(
    public readonly userID: UUID,
    public readonly opCounter: number,
  ) {}

  static key(id: ID): string {
    return `${id.userID}:${id.opCounter}`;
  }

  static equals(a: ID, b: ID): boolean {
    return a.userID === b.userID && a.opCounter === b.opCounter;
  }
}

export type YataItem = {
  id: ID;
  left: ID;
  right: ID;
  originLeft: ID;
  originRight: ID;
  deleted: boolean;
  content: unknown;
}

export type YataStateVector = Map<UUID, number>;

export interface YataType {
  getStateVector(): YataStateVector;
  applyUpdate(update: Uint8Array): void;
  encodeStateVector(): Uint8Array;
  encodeStateAsUpdate(encodedTargetStateVector: Uint8Array): Uint8Array;
}
