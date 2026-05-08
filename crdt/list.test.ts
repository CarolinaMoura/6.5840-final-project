import { describe, it, expect } from "vitest";
import { YataList } from "./list";

function getListAsString(list: YataList): string {
  return list.getItems().map(String).join("");
}

/**
 * Clones a value by serializing it to JSON and then parsing it back.
 * This is used in the tests to create deep copies of the updates since
 * they are live references on the source peer's list. This does NOT matter
 * in the P2P sync protocol since the updates are always encoded and sent over the wire,
 * NOT passed to another peer locally.
 * @param value - The value to clone.
 * @returns The cloned value.
 */
function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value));
}

describe("YataList", () => {
  it("basic insert", () => {
    const list = new YataList(0);
    list.insertItem(0, "Y");
    list.insertItem(1, "A");
    list.insertItem(2, "T");
    list.insertItem(3, "A");

    expect(list.length()).toBe(4);
    expect(list.getItems()).toEqual(["Y", "A", "T", "A"]);
    expect(getListAsString(list)).toBe("YATA");
  });

  it("delete item", () => {
    const list = new YataList(0);
    list.insertItem(0, "A");
    list.insertItem(1, "B");
    list.insertItem(2, "C");
    list.deleteItem(1);

    expect(list.length()).toBe(2);
    expect(getListAsString(list)).toBe("AC");
  });

  it("state vector", () => {
    const list = new YataList(7);
    expect(list.getStateVector().get(7) ?? 0).toBe(0);

    list.insertItem(0, "x");
    list.insertItem(1, "y");
    const afterInserts = list.getStateVector().get(7) ?? 0;
    expect(afterInserts).toBeGreaterThanOrEqual(2);

    list.deleteItem(0);
    expect((list.getStateVector().get(7) ?? 0)).toBeGreaterThanOrEqual(afterInserts);
  });

  it("merge basic : assumes owner id tie breaks", () => {
    const list0 = new YataList(0);
    list0.insertItem(0, "Y");
    list0.insertItem(1, "A");

    const list1 = new YataList(1);
    list1.insertItem(0, "T");
    list1.insertItem(1, "A");

    const update = list0.makeUpdate();
    list1.applyUpdate(clone(update));

    expect(getListAsString(list1)).toBe("YATA");
  });

  it("delta sync", () => {
    const a = new YataList(0);
    a.insertItem(0, "H");
    a.insertItem(1, "I");

    const b = new YataList(1);
    b.applyUpdate(clone(a.makeUpdate()));
    expect(getListAsString(b)).toBe("HI");

    a.insertItem(2, "!");
    const delta = a.makeUpdate(b.getStateVector());
    b.applyUpdate(clone(delta));

    expect(getListAsString(b)).toBe("HI!");
  });

  it("concurrent edits converge on both replicas after bidirectional sync", () => {
    const a = new YataList(0);
    const b = new YataList(1);

    a.insertItem(0, "X");
    b.insertItem(0, "Y");

    const aSV = a.getStateVector();
    const bSV = b.getStateVector();
    a.applyUpdate(clone(b.makeUpdate(aSV)));
    b.applyUpdate(clone(a.makeUpdate(bSV)));

    expect(a.length()).toBe(2);
    expect(b.length()).toBe(2);
    expect(getListAsString(a)).toBe(getListAsString(b));
  });
 
  it("propagates tombstones via the deletes channel after both peers have the item", () => {
    const a = new YataList(0);
    const b = new YataList(1);
    a.insertItem(0, "X");
    b.applyUpdate(clone(a.makeUpdate()));
    expect(getListAsString(b)).toBe("X");

    a.deleteItem(0);
    b.applyUpdate(clone(a.makeUpdate(b.getStateVector())));
    b.applyTombstones(a.getTombstoneKeys());
    expect(b.length()).toBe(0);
  });

  it("merge handles cross-user origin dependencies", () => {
    const a = new YataList(0);
    const b = new YataList(1);

    a.insertItem(0, "X");

    b.applyUpdate(clone(a.makeUpdate()));
    b.insertItem(1, "Y");

    a.applyUpdate(clone(b.makeUpdate(a.getStateVector())));
    a.insertItem(2, "Z");

    expect(getListAsString(a)).toBe("XYZ");

    const c = new YataList(2);
    c.applyUpdate(clone(a.makeUpdate()));
    expect(getListAsString(c)).toBe("XYZ");
  });
});
