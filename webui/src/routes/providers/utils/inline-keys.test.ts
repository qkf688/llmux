import { describe, expect, it } from "vitest";
import {
  deriveInlineKeyStats,
  inlineKeysToArray,
  maskKey,
  splitInlineKeys,
} from "./inline-keys";

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
});

describe("inlineKeysToArray - 表单值 → 提交数组（保行边界）", () => {
  it("换行拆回行边界，trim 去空", () => {
    expect(inlineKeysToArray("sk-a\n sk-b \n\n")).toEqual(["sk-a", "sk-b"]);
  });

  it("单行显式内容不被按逗号/空格二次拆散（行内内容即最终内容）", () => {
    expect(inlineKeysToArray("sk-a sk-b")).toEqual(["sk-a sk-b"]);
  });

  it("CRLF（\\r\\n）粘贴也能正确拆行（trim 吞掉 \\r）", () => {
    expect(splitInlineKeys("sk-a\r\nsk-b\r\n")).toEqual(["sk-a", "sk-b"]);
    expect(inlineKeysToArray("sk-a\r\nsk-b")).toEqual(["sk-a", "sk-b"]);
  });

  it("只有空行 → 空数组（后端 normalizeInlineKeys 兜底校验）", () => {
    expect(inlineKeysToArray("\n  \n")).toEqual([]);
  });
});

describe("maskKey - 掩码显示", () => {
  it("保留头尾 4 字符，中间掩码", () => {
    expect(maskKey("sk-abcdefghijklmnopWXYZ")).toBe("sk-a••••WXYZ");
  });

  it("短 key（≤8 字符）全掩", () => {
    expect(maskKey("abcdef")).toBe("••••••");
  });

  it("8/9 字符边界：8 全掩，9 起保留头尾 4 字符", () => {
    expect(maskKey("abcdefgh")).toBe("••••••");
    expect(maskKey("abcdefghi")).toBe("abcd••••fghi");
  });

  it("空串 → 空", () => {
    expect(maskKey("  ")).toBe("");
  });
});

describe("deriveInlineKeyStats - 统计与空行警示判定", () => {
  it("正常多行：count/totalLines 正确、无空行、无重复", () => {
    const s = deriveInlineKeyStats("sk-a\nsk-b\nsk-c");
    expect(s.count).toBe(3);
    expect(s.totalLines).toBe(3);
    expect(s.duplicates).toBe(0);
    expect(s.hasBlankLine).toBe(false);
    expect(s.allBlank).toBe(false);
  });

  it("重复条目计入 duplicates（trim 后判重）", () => {
    const s = deriveInlineKeyStats("sk-a\n sk-a\nsk-b");
    expect(s.count).toBe(3);
    expect(s.duplicates).toBe(1);
  });

  it("中间空行（部分解密失败/手动空行）→ hasBlankLine 且非 allBlank", () => {
    const s = deriveInlineKeyStats("sk-a\n\nsk-c");
    expect(s.hasBlankLine).toBe(true);
    expect(s.allBlank).toBe(false);
    expect(s.count).toBe(2);
    expect(s.totalLines).toBe(3);
  });

  it("整组空行（全量解密失败形态：N-1 个换行）→ allBlank、totalLines 即原凭据条数", () => {
    const s = deriveInlineKeyStats("\n\n");
    expect(s.allBlank).toBe(true);
    expect(s.hasBlankLine).toBe(true);
    expect(s.totalLines).toBe(3);
    expect(s.count).toBe(0);
  });

  it("全新空组（value 为空串）不算空行警示", () => {
    const s = deriveInlineKeyStats("");
    expect(s.hasBlankLine).toBe(false);
    expect(s.allBlank).toBe(false);
    expect(s.totalLines).toBe(0);
  });
});