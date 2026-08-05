// 三态 supports_thinking 的表单枚举 ↔ 提交值互转（单一转换点，禁止在 hooks/dialog 里手写散落转换）。

/** "inherit" | "true" | "false"：表单三态枚举（inherit=继承 model） */
export type ThinkingTriState = "inherit" | "true" | "false";

/** 关联字段（boolean | null）→ 表单枚举：null/undefined=继承，true/false=override */
export const toTriState = (value: boolean | null | undefined): ThinkingTriState => {
  if (value === null || value === undefined) return "inherit";
  return value ? "true" : "false";
};

/** 表单枚举 → 提交值：inherit=undefined（不传该字段，后端置 NULL），true/false=override */
export const fromTriState = (value: ThinkingTriState): boolean | undefined => {
  if (value === "inherit") return undefined;
  return value === "true";
};
