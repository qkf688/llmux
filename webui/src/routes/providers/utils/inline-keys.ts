/**
 * 内联 key 的纯函数集（inline-keys-field 组件 + buildProviderPayload 共用，单一分词源）。
 *
 * 两个拆分词法有明确分工，勿混用：
 * - splitInlineKeys：批量粘贴入口用，自适应 换行/逗号/空格/制表 分隔（用户从表格/文本批量复制时
 *   常见分隔符不确定）。key 本身不含这些字符，自适应拆分是「所见即所得」的前提——拆坏会在行
 *   列表直接显形，而非静默落库。
 * - inlineKeysToArray：表单值 → 提交数组用。表单值恒为换行分隔（组件内部行 join("\n")），
 *   按换行拆回即行边界；逗号/空格已在粘贴入口拆散，此处再做会破坏单行输入的显式内容。
 */

/** 批量粘贴拆分：按 换行/逗号/空格/制表 分隔 + trim + 去空（保留重复，供统计提示） */
export function splitInlineKeys(text: string): string[] {
  return text
    .split(/[\n, \t]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

/** 表单值 → 提交数组：按换行拆行边界 + trim + 去空（对齐后端 normalizeInlineKeys 的空行兜底） */
export function inlineKeysToArray(text: string): string[] {
  return text
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean);
}

/** 掩码显示：短 key 全掩；常规 key 保留头尾 4 字符。展示用，不参与数据。 */
export function maskKey(key: string): string {
  const trimmed = key.trim();
  if (trimmed.length === 0) {
    return "";
  }
  if (trimmed.length <= 8) {
    return "••••••";
  }
  return `${trimmed.slice(0, 4)}••••${trimmed.slice(-4)}`;
}

export type InlineKeyStats = {
  /** 非空条目数 */
  count: number;
  /** 重复条数（非空 trim 后判重） */
  duplicates: number;
  /** 原始行数，含空行。回填场景 = 后端 InlineKeys 数组长度，用于推算「无法显示的条目数」 */
  totalLines: number;
  /** 存在空行 */
  hasBlankLine: boolean;
  /** 全部为空行（且至少一行）——「整组凭据无法显示 / 全空输入」状态 */
  allBlank: boolean;
};

/**
 * 从表单值（换行分隔）派生统计。空行来源有二：用户显式输入的空行；后端解密失败回空串
 * （join 后占一行）。两者在保存时的行为一致（空白条目被忽略），因此同一套统计兼作
 * 「解密失败 / 空行」警示的判定依据。
 */
export function deriveInlineKeyStats(text: string): InlineKeyStats {
  // 全新空组（value=""）不是「空行」语义：一行都没有，直接短路，避免 split("\n") 给的 [""] 误判
  if (text === "") {
    return { count: 0, duplicates: 0, totalLines: 0, hasBlankLine: false, allBlank: false };
  }
  const lines = text.split("\n");
  const nonEmpty = lines.map((l) => l.trim()).filter(Boolean);
  const unique = new Set(nonEmpty);
  return {
    count: nonEmpty.length,
    duplicates: nonEmpty.length - unique.size,
    totalLines: lines.length,
    hasBlankLine: lines.some((l) => l.trim() === ""),
    allBlank: lines.length > 0 && lines.every((l) => l.trim() === ""),
  };
}