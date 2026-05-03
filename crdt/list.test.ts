import { describe, it, expect } from "vitest";
import { YataList } from "./list";

function getListAsString(list: YataList): string {
  return list.getItems().map(String).join("");
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

  it("merge basic", () => {
    const list0 = new YataList(0);
    list0.insertItem(0, "Y");
    list0.insertItem(1, "A");

    const list1 = new YataList(1);
    list1.insertItem(0, "T");
    list1.insertItem(1, "A");

    const update = list0.encodeStateAsUpdate();
    list1.applyUpdate(update);

    expect(getListAsString(list1)).toBe("YATA");
  });

  it("delta sync", () => {
    const a = new YataList(0);
    a.insertItem(0, "H");
    a.insertItem(1, "I");

    const b = new YataList(1);
    b.applyUpdate(a.encodeStateAsUpdate());
    expect(getListAsString(b)).toBe("HI");

    a.insertItem(2, "!");
    const delta = a.encodeStateAsUpdate(b.encodeStateVector());
    b.applyUpdate(delta);

    expect(getListAsString(b)).toBe("HI!");
  });

  it("concurrent edits converge on both replicas after bidirectional sync", () => {
    const a = new YataList(0);
    const b = new YataList(1);

    a.insertItem(0, "X");
    b.insertItem(0, "Y");

    const aSV = a.encodeStateVector();
    const bSV = b.encodeStateVector();
    a.applyUpdate(b.encodeStateAsUpdate(aSV));
    b.applyUpdate(a.encodeStateAsUpdate(bSV));

    expect(a.length()).toBe(2);
    expect(b.length()).toBe(2);
    expect(getListAsString(a)).toBe(getListAsString(b));
  });
});
