import type { TestMode } from "@/lib/api";

export type {
  ProviderModelGroup,
  ProviderModelWithOwner,
} from "@/lib/provider-models";
// 模式与流式诊断块是回包契约，定义在 API 层；此处转出一并给页面/弹窗用。
export type { StreamTestStats, TestMode } from "@/lib/api";

export type ProviderModelSelection = {
  providerId: number;
  providerName: string;
  modelId: string;
};

export type BlacklistFilter = "all" | "blacklisted" | "not-blacklisted";

// 「protocol」是唯一没有模式轴的一类：一次调用内部已把非流式与流式都跑完（后端端点无
// ?stream= 参数），弹窗对它禁用模式开关。
export type TestType = "connectivity" | "react" | "structured_output" | "thinking" | "vision" | "protocol";

// 能力测试结果的存储键：关联 id + 模式。分开存是因为「非流式通过、流式失败」是两个
// 都成立且都要看见的结论，共用一格会让切换模式后读到上一个模式的旧结果。
export type CapabilityTestKey = `${number}:${TestMode}`;

export type BatchTestProgress = {
  total: number;
  completed: number;
  success: number;
  failed: number;
  /** 被中止的项数。与 failed 分开计数，失败率不被取消污染。 */
  cancelled: number;
  testing: number;
};

export type AssociationBatchTestResult = {
  loading: boolean;
  success: boolean | null;
  error?: string;
  /** 用户主动中止（批量取消 / 切页面）或登录过期。与「失败」分开：两者都不证明该关联不可用。 */
  cancelled?: boolean;
};

export type ReactTestResultState = {
  loading: boolean;
  messages: string;
  success: boolean | null;
  error: string | null;
  /** 关弹窗/切页面中止。已累积的半截日志保留，供下次打开对照。 */
  cancelled?: boolean;
};

export type ModelProvidersPagePreferences = {
  searchKeyword: string;
  selectedProviderType: string;
  selectedProviderFilter: string;
  selectedStatusFilter: string;
  filterPanelOpen: boolean;
  operationScope: "current" | "all";
};

