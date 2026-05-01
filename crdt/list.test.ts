import { describe, it, expect } from "vitest";
import { YataList } from "./list";

function getListAsString(list: YataList): string {
  throw new Error("not implemented");
}

describe("YataList", () => {
  it("basic insert", () => {
    const list = new YataList(0);
    list.insertItem(0, "Y");
    list.insertItem(1, "A");
    list.insertItem(2, "T");
    list.insertItem(3, "A");

    expect(getListAsString(list)).toBe("YATA");
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
});
