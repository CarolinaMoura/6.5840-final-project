import { ID, UUID, YataItem, YataStateVector, YataType } from "./types";

const SENTINEL_USER: UUID = -1;
const HEAD_ID: ID = new ID(SENTINEL_USER, 0);
const TAIL_ID: ID = new ID(SENTINEL_USER, 1);

type WireUpdate = {
  items: YataItem[];
};

type WireStateVector = Record<string, number>;

export class YataList implements YataType {
  private readonly ownerID: UUID;
  private opCounter: number;
  private readonly itemById: Map<string, YataItem>;
  private readonly head: YataItem;
  private readonly tail: YataItem;
  private readonly stateVector: YataStateVector;

  constructor(uuid: UUID) {
    this.ownerID = uuid;
    this.opCounter = 0;
    this.itemById = new Map<string, YataItem>();
    this.stateVector = new Map<UUID, number>();
    this.stateVector.set(uuid, 0);

    this.head = {
      id: HEAD_ID,
      left: HEAD_ID,
      right: TAIL_ID,
      originLeft: HEAD_ID,
      originRight: HEAD_ID,
      deleted: true,
      content: null,
    };
    this.tail = {
      id: TAIL_ID,
      left: HEAD_ID,
      right: TAIL_ID,
      originLeft: TAIL_ID,
      originRight: TAIL_ID,
      deleted: true,
      content: null,
    };
    this.itemById.set(ID.key(HEAD_ID), this.head);
    this.itemById.set(ID.key(TAIL_ID), this.tail);
  }

  private isSentinel(item: YataItem): boolean {
    return item.id.userID === SENTINEL_USER;
  }

  private getItem(id: ID): YataItem {
    const item = this.itemById.get(ID.key(id));
    if (!item) {
      throw new Error(`YataList: missing item with id ${ID.key(id)}`);
    }
    return item;
  }

  private bumpStateVector(id: ID): void {
    const next = id.opCounter + 1;
    const current = this.stateVector.get(id.userID) ?? 0;
    if (next > current) {
      this.stateVector.set(id.userID, next);
    }
  }

  getStateVector(): YataStateVector {
    // returns a copy to prevent mutation of the internal state vector.
    return new Map(this.stateVector);
  }
 
  private integrate(item: YataItem): void {
    let left = this.getItem(item.originLeft);
    const right = this.getItem(item.originRight);

    let scan = this.getItem(left.right);
    const conflictingItems = new Set<string>();
    const itemsBeforeOrigin = new Set<string>();

    while (!ID.equals(scan.id, right.id)) {
      itemsBeforeOrigin.add(ID.key(scan.id));
      conflictingItems.add(ID.key(scan.id));

      if (ID.equals(scan.originLeft, item.originLeft)) {
        // tie-break by UUID
        if (scan.id.userID < item.id.userID) {
          left = scan;
          conflictingItems.clear();
        } else if (ID.equals(scan.originRight, item.originRight)) {
          break;
        }
      } else if (itemsBeforeOrigin.has(ID.key(scan.originLeft))) {
        if (!conflictingItems.has(ID.key(scan.originLeft))) {
          left = scan;
          conflictingItems.clear();
        }
      } else {
        break;
      }

      scan = this.getItem(scan.right);
    }

    const rightNeighbor = this.getItem(left.right);
    item.left = left.id;
    item.right = rightNeighbor.id;
    left.right = item.id;
    rightNeighbor.left = item.id;

    this.itemById.set(ID.key(item.id), item);
    this.bumpStateVector(item.id);
  }

  applyUpdate(update: Uint8Array): void {
    const wire = JSON.parse(new TextDecoder().decode(update)) as WireUpdate;
    const pending: YataItem[] = wire.items ?? [];

    // Items may arrive out of dependency order (an item's origins may not yet
    // exist locally). Repeatedly integrate everything whose origins are known,
    // until the queue drains or no progress can be made.
    let progress = true;
    while (pending.length > 0 && progress) {
      progress = false;
      for (let i = pending.length - 1; i >= 0; i--) {
        const item = pending[i];
        const olKey = ID.key(item.originLeft);
        const orKey = ID.key(item.originRight);
        if (!this.itemById.has(olKey) || !this.itemById.has(orKey)) {
          continue;
        }

        const existingKey = ID.key(item.id);
        const existing = this.itemById.get(existingKey);
        if (existing) {
          // Already integrated; merge tombstone state monotonically.
          if (item.deleted) existing.deleted = true;
        } else {
          this.integrate(item);
        }
        pending.splice(i, 1);
        progress = true;
      }
    }

    if (pending.length > 0) {
      throw new Error(
        `YataList.applyUpdate: ${pending.length} item(s) had unresolved origins`
      );
    }
  }

  encodeStateVector(): Uint8Array {
    const obj: WireStateVector = {};
    for (const [user, counter] of this.stateVector) {
      obj[String(user)] = counter;
    }
    return new TextEncoder().encode(JSON.stringify(obj));
  }

  encodeStateAsUpdate(encodedTargetStateVector?: Uint8Array): Uint8Array {
    const target: YataStateVector = new Map<UUID, number>();
    if (encodedTargetStateVector && encodedTargetStateVector.byteLength > 0) {
      const obj = JSON.parse(
        new TextDecoder().decode(encodedTargetStateVector)
      ) as WireStateVector;
      for (const [user, counter] of Object.entries(obj)) {
        target.set(Number(user), counter);
      }
    }

    const items: YataItem[] = [];
    for (const item of this.itemById.values()) {
      if (this.isSentinel(item)) continue;
      const known = target.get(item.id.userID) ?? 0;
      if (item.id.opCounter >= known) {
        items.push(item);
      }
    }

    // send origins before dependents so the receiver can integrate in one pass when possible
    items.sort((a, b) => {
      if (a.id.userID !== b.id.userID) return a.id.userID - b.id.userID;
      return a.id.opCounter - b.id.opCounter;
    });

    const wire: WireUpdate = { items };
    return new TextEncoder().encode(JSON.stringify(wire));
  }

  /**
  Inserts an item at the given index. 
  If the index is greater than the length of the list, the item is appended to the end of the list.
  @param index - The index at which to insert the item.
  @requires index is a non-negative integer
  @requires content is not null
  **/
  insertItem(index: number, content: unknown): void {
    const visible = this.collectVisibleItems();
    const clampedIndex = Math.min(Math.max(index, 0), visible.length);

    const leftItem = clampedIndex === 0 ? this.head : visible[clampedIndex - 1];
    const rightItem = clampedIndex >= visible.length ? this.tail : visible[clampedIndex];

    const id: ID = new ID(this.ownerID, this.opCounter++);
    const newItem: YataItem = {
      id,
      left: leftItem.id,
      right: rightItem.id,
      originLeft: leftItem.id,
      originRight: rightItem.id,
      deleted: false,
      content,
    };

    this.integrate(newItem);
  }

  /**
  Marks the item at the given index as deleted. The item is not removed from the list.
  @param index - The index of the item to mark as deleted.
  @requires index is a non-negative integer and index < this.length()
  **/
  deleteItem(index: number): void {
    let visibleIndex = 0;
    let cursor = this.getItem(this.head.right);
    while (!ID.equals(cursor.id, TAIL_ID)) {
      if (!cursor.deleted) {
        if (visibleIndex === index) {
          cursor.deleted = true;
          return;
        }
        visibleIndex++;
      }
      cursor = this.getItem(cursor.right);
    }
    throw new Error(
      `YataList.deleteItem: index ${index} out of range (length ${visibleIndex})`
    );
  }

  /**
  Length of the list.
  @returns the length of the list.
  **/
  length(): number {
    let count = 0;
    let cursor = this.getItem(this.head.right);
    while (!ID.equals(cursor.id, TAIL_ID)) {
      if (!cursor.deleted) count++;
      cursor = this.getItem(cursor.right);
    }
    return count;
  }

  /**
  Returns the items in the list.
  @returns an array of the items in the list.
  **/
  getItems(): unknown[] {
    return this.collectVisibleItems().map((item) => item.content);
  }

  private collectVisibleItems(): YataItem[] {
    const out: YataItem[] = [];
    let cursor = this.getItem(this.head.right);
    while (!ID.equals(cursor.id, TAIL_ID)) {
      if (!cursor.deleted) out.push(cursor);
      cursor = this.getItem(cursor.right);
    }
    return out;
  }
}
