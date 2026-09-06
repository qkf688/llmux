import { describe, expect, it } from "vitest";
import { deriveInlineKeyStats, splitInlineKeys } from "./inline-keys";
import type { ProviderFormInlineKeyRow } from "../form-schema";

/** 表单行辅助：明文行（可为空占位）/ 掩码行（服务端掩码文本回填形态） */
function plain(key = "", id?: number): ProviderFormInlineKeyRow {
  return { id, key, masked: false };
}
function masked(key: string, id: number): ProviderFormInlineKeyRow {
  return { id, key, masked: true };
}

describe("splitInlineKeys - 批量粘贴拆分（自适应分隔符）", () => {
  it("换行分隔拆成多行、trim 去空行", () => {
    expect(splitInlineKeys(" sk-a \n sk-b \n ")).toEqual(["sk-a", "sk-b"]);
  });

  it("逗号分隔拆分（表格粘贴常见）", () => {
    expect(splitInlineKeys("sk-a, sk-b,sk-c")).toEqual(["sk-a", "sk-b", "sk-c"]);
  });

  it("空格/制表分隔拆分（文本粘贴常见，修复静默并 key 的问题）", () => {
    expect(splitInlineKeys("sk-a sk-b\tsk-c")).toEqual(["sk-a", "sk-b", "sk-c"]);
  });

  it("混合分隔符 + 连续空白 collapse", () => {
    expect(splitInlineKeys("sk-a sk-b,\tsk-c\n\n sk-d")).toEqual(["sk-a", "sk-b", "sk-c", "sk-d"]);
  });

  it("保留重复（去重交由统计提示，不静默吞）", () => {
    expect(splitInlineKeys("sk-a\nsk-a")).toEqual(["sk-a", "sk-a"]);
  });

  it("空串/纯空白 → 空数组", () => {
    expect(splitInlineKeys("")).toEqual([]);
    expect(splitInlineKeys("  \t\n  ")).toEqual([]);
  });

  it("CRLF（\\r\\n）粘贴也能正确拆行（trim 吞掉 \\r）", () => {
    expect(splitInlineKeys("sk-a\r\nsk-b\r\n")).toEqual(["sk-a", "sk-b"]);
  });
});

describe("deriveInlineKeyStats - 行数组统计与空态警示判定", () => {
  it("纯明文多行：count 正确、无空行、无重复", () => {
    const s = deriveInlineKeyStats([plain("sk-a"), plain("sk-b"), plain("sk-c")]);
    expect(s.count).toBe(3);
    expect(s.duplicates).toBe(0);
    expect(s.hasBlankLine).toBe(false);
    expect(s.allBlank).toBe(false);
  });

  it("掩码行计入 count、不参与重复/空态判定", () => {
    const s = deriveInlineKeyStats([masked("sk-a****1111", 1), plain("sk-a"), plain("sk-b")]);
    expect(s.count).toBe(3);
    expect(s.duplicates).toBe(0);
    expect(s.hasBlankLine).toBe(false);
    expect(s.allBlank).toBe(false);
  });

  it("重复条目计入 duplicates（明文行 trim 后判重，掩码行不参与）", () => {
    const s = deriveInlineKeyStats([plain("sk-a"), plain(" sk-a "), plain("sk-b")]);
    expect(s.count).toBe(3);
    expect(s.duplicates).toBe(1);
  });

  it("存在空明文占位行 → hasBlankLine 且非 allBlank", () => {
    const s = deriveInlineKeyStats([plain("sk-a"), plain(), plain("sk-c")]);
    expect(s.hasBlankLine).toBe(true);
    expect(s.allBlank).toBe(false);
    expect(s.count).toBe(2);
  });

  it("整组空明文行 → allBlank（需重填警示）", () => {
    const s = deriveInlineKeyStats([plain(), plain()]);
    expect(s.allBlank).toBe(true);
    expect(s.hasBlankLine).toBe(true);
    expect(s.count).toBe(0);
  });

  it("全新空组（空数组）→ allBlank（无有效 key）", () => {
    const s = deriveInlineKeyStats([]);
    expect(s.hasBlankLine).toBe(false);
    expect(s.allBlank).toBe(true);
  });

  it("回填掩码态（未显示）不触发空态警示——有凭据、保存按 id 保留", () => {
    const s = deriveInlineKeyStats([masked("sk-****1111", 1)]);
    expect(s.allBlank).toBe(false);
    expect(s.count).toBe(1);
    expect(s.hasBlankLine).toBe(false);
  });
});
