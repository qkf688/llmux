import { describe, expect, it } from "vitest";
import { fromTriState, toTriState } from "./tri-state";

describe("toTriState（关联字段 → 表单枚举）", () => {
  it("null / undefined → inherit", () => {
    expect(toTriState(null)).toBe("inherit");
    expect(toTriState(undefined)).toBe("inherit");
  });

  it("true → true，false → false", () => {
    expect(toTriState(true)).toBe("true");
    expect(toTriState(false)).toBe("false");
  });
});

describe("fromTriState（表单枚举 → 提交值）", () => {
  it("inherit → undefined（后端置 NULL 实现改回继承）", () => {
    expect(fromTriState("inherit")).toBeUndefined();
  });

  it("true / false → 布尔 override", () => {
    expect(fromTriState("true")).toBe(true);
    expect(fromTriState("false")).toBe(false);
  });
});

describe("往返一致性", () => {
  it("toTriState 与 fromTriState 互为逆", () => {
    for (const v of [null, true, false] as const) {
      expect(fromTriState(toTriState(v))).toBe(v === null ? undefined : v);
    }
  });
});
