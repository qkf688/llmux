import type { ErrorType } from "@/lib/errors";
import type { HealthCheckStatus } from "../system/health-check";
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
  // 能力三态：null=继承 Model 同名真值，true/false=override。
  // 键名为 PascalCase 且后端保证**始终存在**（`models.ModelWithProvider` 直返，禁 omitempty）；
  // 若键名失配读到的会是 undefined，编辑弹窗会把继承误判成 override 并在保存时写坏数据。
  ToolCall: boolean | null;
  StructuredOutput: boolean | null;
  Image: boolean | null;
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

// 健康检测状态值域由健康检测域定义（同一份 HealthCheckLog.status），此处复用而非另立别名
export async function getModelProviderHealthStatus(
  modelProviderId: number,
  limit = 10
): Promise<HealthCheckStatus[]> {
  const params = new URLSearchParams({
    model_provider_id: modelProviderId.toString(),
    limit: limit.toString(),
  });
  return apiRequest<HealthCheckStatus[]>(`/model-providers/health-status?${params.toString()}`);
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
  // 能力三态：null/不传 = 继承 model 的能力真值（默认态）。
  tool_call?: boolean | null;
  structured_output?: boolean | null;
  image?: boolean | null;
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
    // 能力三态：null = 显式清回继承（后端写 NULL），不发字段同样清回继承。
    tool_call?: boolean | null;
    structured_output?: boolean | null;
    image?: boolean | null;
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
// signal 可选透传：取消要真正生效必须一路传到 fetch 层——只在前端把 loading 置否
// 不会停止上游会话，流式一次 20~60 秒都在计费。
const streamModeSuffix = (mode: TestMode) => (mode === "stream" ? "?stream=1" : "");

export async function testModelProvider(
  id: number,
  mode: TestMode = "non_stream",
  signal?: AbortSignal
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/${id}${streamModeSuffix(mode)}`, { signal });
}

export async function testModelProviderStructuredOutput(
  id: number,
  mode: TestMode = "non_stream",
  signal?: AbortSignal
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/structured-output/${id}${streamModeSuffix(mode)}`, { signal });
}

export async function testModelProviderThinking(
  id: number,
  mode: TestMode = "non_stream",
  signal?: AbortSignal
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/thinking/${id}${streamModeSuffix(mode)}`, { signal });
}

export async function testModelProviderVision(
  id: number,
  mode: TestMode = "non_stream",
  signal?: AbortSignal
): Promise<ModelProviderTestResult> {
  return apiRequest<ModelProviderTestResult>(`/test/vision/${id}${streamModeSuffix(mode)}`, { signal });
}

/* ---------- 协议支持检测（第六类测试，只读） ---------- */

/** 单条探针腿的判定。noncompliant = 回了 SSE 但缺终止事件/无内容增量（实现不规范），
 *  与 failed（压根不按流式回 / 连不上）是两类问题，处置不同，故单列。 */
export type ProtocolLegStatus = "passed" | "failed" | "noncompliant";

export interface ProtocolLegResult {
  status: ProtocolLegStatus;
  passed: boolean;
  status_code: number;
  duration_ms: number;
  first_frame_ms?: number;
  terminated_by?: string;
  error_type?: string;
  summary?: string;
  detail?: string;
}

/** 无效模型请求采到的错误信封证据：家族是否与自身协议同族 = 是否像原样直通。 */
export interface ProtocolFingerprint {
  attempted: boolean;
  probe_model: string;
  status_code: number;
  family: string;
  same_family: boolean;
  error_type?: string;
  message?: string;
  native_headers: string[];
}

export interface ProtocolProbeEntry {
  protocol: string;
  provider_type: string;
  /** 完整请求 URL（含协议路径段），展示用。 */
  request_url: string;
  url_source: "endpoint" | "inherited";
  endpoint_configured: boolean;
  verdict: "usable" | "partial" | "unusable";
  non_stream: ProtocolLegResult;
  stream: ProtocolLegResult;
  fingerprint: ProtocolFingerprint;
  /** 该协议「应有」的原生特征命中情况（跨协议不比数量，只看各自完整度）。 */
  fields: { hit: string[]; miss: string[] };
  id_prefix: string;
  recommended: boolean;
}

export interface ProtocolProbeRecommendation {
  /** 空串 = 不点名任何协议（证据不足时的「无法区分」，后端刻意如此）。 */
  protocol: string;
  confidence: "high" | "medium" | "low";
  summary: string;
  reason: string;
  actions: string[];
  notes: string[];
}

export interface ProtocolProbeResult {
  passed?: boolean;
  group_id?: number;
  group_name?: string;
  provider?: string;
  model?: string;
  main_protocol?: string;
  protocols?: ProtocolProbeEntry[];
  recommendation?: ProtocolProbeRecommendation;
}

// 协议检测**没有模式轴**：一次调用内部已经把非流式与流式都跑完，
// 前端不再提供模式开关（后端端点也没有 ?stream= 参数）。
export async function testModelProviderProtocols(id: number, signal?: AbortSignal): Promise<ProtocolProbeResult> {
  return apiRequest<ProtocolProbeResult>(`/test/protocols/${id}`, { signal });
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
