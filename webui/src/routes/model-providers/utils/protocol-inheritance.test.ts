import { describe, expect, it } from "vitest";
import type { ModelWithProvider } from "@/lib/api";
import { isProtocolsInherited } from "./protocol-inheritance";

function association(id: number, protocols: string[] | null): ModelWithProvider {
  return { ID: id, Protocols: protocols } as ModelWithProvider;
}

describe("isProtocolsInherited", () => {
  it("Protocols 为 null = 继承供应商", () => {
    expect(isProtocolsInherited([association(1, null)], 1)).toBe(true);
  });

  it("Protocols 非 null = 已显式覆盖", () => {
    expect(isProtocolsInherited([association(1, ["openai"])], 1)).toBe(false);
    // 空数组在写入侧已归一为 null，理论上不会出现；真出现了也不该判成继承。
    expect(isProtocolsInherited([association(1, [])], 1)).toBe(false);
  });

  it("找不到该关联时不提示（宁可漏提示，不可误报）", () => {
    expect(isProtocolsInherited([association(2, null)], 1)).toBe(false);
    expect(isProtocolsInherited([], 1)).toBe(false);
  });

  it("未选中关联时返回 false", () => {
    expect(isProtocolsInherited([association(1, null)], null)).toBe(false);
  });
});
