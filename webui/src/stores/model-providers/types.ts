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

export type TestType = "connectivity" | "react" | "structured_output" | "thinking" | "vision";

// 能力测试结果的存储键：关联 id + 模式。分开存是因为「非流式通过、流式失败」是两个
// 都成立且都要看见的结论，共用一格会让切换模式后读到上一个模式的旧结果。
export type CapabilityTestKey = `${number}:${TestMode}`;

export type BatchTestProgress = {
  total: number;
  completed: number;
  success: number;
  failed: number;
  testing: number;
};

export type AssociationBatchTestResult = {
  loading: boolean;
  success: boolean | null;
  error?: string;
};

export type ReactTestResultState = {
  loading: boolean;
  messages: string;
  success: boolean | null;
  error: string | null;
};

export type ModelProvidersPagePreferences = {
  searchKeyword: string;
  selectedProviderType: string;
  selectedProviderFilter: string;
  selectedStatusFilter: string;
  filterPanelOpen: boolean;
  operationScope: "current" | "all";
};

