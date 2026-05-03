export type UUID = number;

export type ID = {
  userID: UUID;
  opCounter: number;
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
