// 思考档位选项：6 档有序 + 2 特殊（none=剥离 thinking，auto=由模型决定）
// model-form-dialog 与 association-form-dialog 共用此常量，避免 DRY 违规。
export const THINKING_LEVEL_OPTIONS: { value: string; label: string }[] = [
  { value: "minimal", label: "minimal" },
  { value: "low", label: "low" },
  { value: "medium", label: "medium" },
  { value: "high", label: "high" },
  { value: "xhigh", label: "xhigh" },
  { value: "max", label: "max" },
  { value: "none", label: "none（剥离）" },
  { value: "auto", label: "auto（自动）" },
];
