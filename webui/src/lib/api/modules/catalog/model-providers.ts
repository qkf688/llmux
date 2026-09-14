import type { ErrorType } from "@/lib/errors";
import { apiRequest } from "../../core/client";

// 测试执行模式与流式诊断块是**回包契约**（值域镜像后端 handler/testapi 的
// testModeStream/testModeNonStream 与 streamStatsPayload），故定义在 API 层，
// 由 stores 侧按需导入——反向导入会让 lib/api 依赖 stores。
export type TestMode = "non_stream" | "stream";

export type StreamTestStats = {
  frames: number;
  first_frame_ms: number;
  duration_ms: number;
  terminated_by: string;
};

export interface ModelWithProvider {
  ID: number;
  ModelID: number;
  ProviderModel: string;
  ProviderID: number;
  ToolCall: boolean;
  StructuredOutput: boolean;
  Image: boolean;
  WithHeader: boolean;
  CustomerHeaders: Record<string, string> | null;
  Status: boolean | null;
  Weight: number;
  Priority: number;
  MaxTokens: number | null;
  SupportsThinking: boolean | null; // 三态：null=继承 model，true/false=override
  // 三态：null=继承 model，[]=不约束，非空=override。
  // 键名为 PascalCase 且后端保证**始终存在**（`models.ModelWithProvider` 直返，禁 omitempty）；
  // 若此处键名与后端 tag 失配，读到的会是 undefined，编辑弹窗会把继承误判成自定义。
  ThinkingLevels: string[] | null;
}

export interface AssociationPreview {
  model_id: number;
  model_name: string;
  provider_id: number;
  provider_name: string;
  provider_model: string;
}

export interface ModelProviderTestResult {
  passed?: boolean;
  error?: string;
  error_type?: ErrorType;
  /** 本次执行的模式（后端恒回传；值域见本文件顶部的 TestMode） */
  mode?: TestMode;
  /** 流式诊断块：仅流式模式且已进入分帧阶段时存在（非流式、或上游连 SSE 都没起时缺席） */
  stream?: StreamTestStats;
  /** 后端实际命中的分组（#19 起与 /providers/:id/test 同形状；未选定链路 = 0/空串） */
  group_id?: number;
  group_name?: string;
  message?: string;
  raw_output?: string;
  parsed?: unknown;
  /** thinking 能力测试提取出的推理内容 */
  thinking_text?: string;
  /** 图片识别能力测试提取出的模型回答文本 */
  vision_text?: string;
  [key: string]: unknown;
}

export async function getModelProviders(modelId: number): Promise<ModelWithProvider[]> {
  return apiRequest<ModelWithProvider[]>(`/model-providers?model_id=${modelId}`);
}

export async function getModelProviderHealthStatus(
  modelProviderId: number,
  limit = 10
): Promise<boolean[]> {
  const params = new URLSearchParams({
    model_provider_id: modelProviderId.toString(),
    limit: limit.toString(),
  });
  return apiRequest<boolean[]>(`/model-providers/health-status?${params.toString()}`);
}

export async function getModelProviderStatus(
  providerId: number,
  modelName: string,
  providerModel: string
): Promise<boolean[]> {
  const params = new URLSearchParams({
    provider_id: providerId.toString(),
    model_name: modelName,
    provider_model: providerModel,
  });
  return apiRequest<boolean[]>(`/model-providers/status?${params.toString()}`);
}

export async function createModelProvider(association: {
  model_id: number;
  provider_model: string;
  provider_id: number;
  tool_call: boolean;
  structured_output: boolean;
  image: boolean;
  with_header: boolean;
  customer_headers: Record<string, string>;
  weight: number;
  priority?: number;
  max_tokens?: number;
  supports_thinking?: boolean;
  thinking_levels?: string[] | null;
}): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>("/model-providers", {
    method: "POST",
    body: JSON.stringify(association),
  });
}

export async function updateModelProvider(
  id: number,
  association: {
    model_id?: number;
    provider_model?: string;
    provider_id?: number;
    tool_call?: boolean;
    structured_output?: boolean;
    image?: boolean;
    with_header?: boolean;
    customer_headers?: Record<string, string>;
    weight?: number;
    priority?: number;
    max_tokens?: number;
    supports_thinking?: boolean;
    thinking_levels?: string[] | null;
  }
): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}`, {
    method: "PUT",
    body: JSON.stringify(association),
  });
}

export async function updateModelProviderStatus(id: number, status: boolean): Promise<ModelWithProvider> {
  return apiRequest<ModelWithProvider>(`/model-providers/${id}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export async function deleteModelProvider(id: number): Promise<void> {
  await apiRequest<void>(`/model-providers/${id}`, {
    method: "DELETE",
  });
}

export async function batchDeleteModelProviders(ids: number[]): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/model-providers/batch", {
    method: "DELETE",
    body: JSON.stringify({ ids }),
  });
}

export async function batchUpdateModelProvidersStatus(
  ids: number[],
  status: boolean
): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/model-providers/batch/status", {
    method: "PATCH",
    body: JSON.stringify({ ids, status }),
  });
}

export async function batchUpdateModelProvidersCapabilities(
  ids: number[],
  capabilities: {
    tool_call?: boolean;
    structured_output?: boolean;
    image?: boolean;
  }
): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/model-providers/batch/capabilities", {
    method: "PATCH",
    body: JSON.stringify({ ids, ...capabilities }),
  });
}

// 四个能力测试端点的模式参数：流式靠 ?stream=1 表达（同一 GET 承载两种模式，
// 后端不新增路由）。非流式不带参数，请求与改造前逐字节一致。
const streamModeSuffix = (mode: TestMode) => (mode === "stream" ? "?stream=1" : "");

export async function testModelProvider(id: number, mode: TestMode = "non_stream"): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/${id}${streamModeSuffix(mode)}`);
}

export async function testModelProviderStructuredOutput(
  id: number,
  mode: TestMode = "non_stream"
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/structured-output/${id}${streamModeSuffix(mode)}`);
}

export async function testModelProviderThinking(
  id: number,
  mode: TestMode = "non_stream"
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/thinking/${id}${streamModeSuffix(mode)}`);
}

export async function testModelProviderVision(
  id: number,
  mode: TestMode = "non_stream"
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/vision/${id}${streamModeSuffix(mode)}`);
}

export async function previewAutoAssociate(): Promise<AssociationPreview[]> {
  return apiRequest<AssociationPreview[]>("/model-providers/auto-associate/preview");
}

export async function autoAssociateModels(): Promise<{ added: number; failed: number }> {
  return apiRequest<{ added: number; failed: number }>("/model-providers/auto-associate", {
    method: "POST",
  });
}

export async function getProviderBlacklist(): Promise<{ blacklisted_ids: number[] }> {
  return apiRequest<{ blacklisted_ids: number[] }>("/providers/blacklist");
}

export async function updateProviderBlacklist(providerIds: number[]): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/providers/blacklist", {
    method: "PUT",
    body: JSON.stringify({ provider_ids: providerIds }),
  });
}

export async function previewCleanInvalid(): Promise<AssociationPreview[]> {
  return apiRequest<AssociationPreview[]>("/model-providers/clean-invalid/preview");
}

export async function cleanInvalidAssociations(): Promise<{ removed: number; failed: number }> {
  return apiRequest<{ removed: number; failed: number }>("/model-providers/clean-invalid", {
    method: "POST",
  });
}
