export type UUID = number;

export interface ID {
  userID: UUID;
  opCounter: number;
}

export interface YataItem {
  id: ID;
  left: ID;
  right: ID;
  originLeft: ID;
  originRight: ID;
  deleted: boolean;
  content: unknown;
}

export type YataStateVector = ID[];

export interface YataType {
  getStateVector(): YataStateVector;
  applyUpdate(update: Uint8Array): void;
  encodeStateVector(): Uint8Array;
  encodeStateAsUpdate(encodedTargetStateVector: Uint8Array): Uint8Array;
}
