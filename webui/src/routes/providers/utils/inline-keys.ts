/** 内联 key 行表单态/工具的纯函数集（inline-keys-field 组件 + detailToFormValues 共用）。

 * 行形态（ProviderFormInlineKeyRow）：
 * - masked=true：服务端掩码文本（只读占位），提交时按 id 保留；
 * - masked=false：明文（可编辑），提交时 key 原样传给后端。
 * splitInlineKeys 只用于批量粘贴入口（自适应分隔符拆分）；提交不再按行拆分——
 * 行 id + key 数组直接映射到后端 InlineKeyInput。
 */
import type { ProviderFormInlineKeyRow } from "../form-schema";

/** 批量粘贴拆分：按 换行/逗号/空格/制表 分隔 + trim + 去空（保留重复，供统计提示） */
export function splitInlineKeys(text: string): string[] {
  return text
    .split(/[\n, \t]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export type InlineKeyStats = {
  /** 展示条目数（掩码行 + 明文非空行） */
  count: number;
  /** 明文行之间 trim 后重复数（掩码行掩码文本不可比，不参与） */
  duplicates: number;
  /** 存在空明文行（待填写占位；保存时忽略，与后端 normalize 空条目兜底一致） */
  hasBlankLine: boolean;
  /** 无掩码行且明文全空——「整组无有效 key，需填写才能保存」状态 */
  allBlank: boolean;
};

/**
 * 从表单行数组派生统计。掩码行是「未拉明文」的既有凭据（提交按 id 保留），
 * 不计入空态；空明文行与新增前的空占位行需区分展示（警示由调用方决定）。
 */
export function deriveInlineKeyStats(rows: ProviderFormInlineKeyRow[]): InlineKeyStats {
  const masked = rows.filter((r) => r.masked).length;
  const keys = rows.filter((r) => !r.masked).map((r) => r.key.trim()).filter(Boolean);
  const emptyRows = rows.filter((r) => !r.masked && r.key.trim() === "").length;
  const unique = new Set(keys);
  return {
    count: masked + keys.length,
    duplicates: keys.length - unique.size,
    hasBlankLine: emptyRows > 0,
    allBlank: masked === 0 && keys.length === 0,
  };
}
