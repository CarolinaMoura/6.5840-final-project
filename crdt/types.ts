export type UUID = number;

export class ID {
  constructor(
    public readonly userID: UUID,
    public readonly opCounter: number,
  ) {}

  /**
  @param id to get the key of.
  @returns a stringified representation of the ID.
  **/
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

export class YataStateVector extends Map<UUID, number> {
  /**
  Builds a YataStateVector from the wire representation
  @param wire - the JSON-decoded wire form.
  @returns a YataStateVector.
  **/
  static fromWire(wire: Record<string, number>): YataStateVector {
    const stateVector = new YataStateVector();
    for (const [user, counter] of Object.entries(wire)) {
      stateVector.set(Number(user), counter);
    }
    return stateVector;
  }

  /**
  Returns the wire representation, suitable for JSON.stringify.
  @returns a Record keyed by stringified UUID.
  **/
  getWire(): Record<string, number> {
    const wire: Record<string, number> = {};
    for (const [user, counter] of this) {
      wire[String(user)] = counter;
    }
    return wire;
  }
}

export interface YataType {
  getStateVector(): YataStateVector;
  applyUpdate(items: YataItem[]): void;
  makeUpdate(targetStateVector?: YataStateVector): YataItem[];
}
