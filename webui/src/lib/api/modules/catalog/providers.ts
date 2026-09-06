import { apiRequest } from "../../core/client";

export interface Provider {
  ID: number;
  Name: string;
  Type: string;
  Config: string;
  Console: string;
  Proxy: string;
  ModelEndpoint?: boolean;
  ModelFilterEnabled?: boolean;
  AuthType?: string;
  blacklisted?: boolean;
  /** 勾选的出站协议（openai/anthropic/responses），来自后端 models.Provider.Protocols */
  Protocols?: string[];
  /** 列表徽标：端点/分组计数（GET /providers 列表项附带） */
  EndpointCount?: number;
  GroupCount?: number;
}

/** 协议端点（详情路径，PascalCase 对齐后端 models.Endpoint） */
export interface ProviderEndpoint {
  ID: number;
  ProviderID: number;
  Protocol: string;
  URL: string;
  Enabled: boolean;
}

/** 内联凭据详情项：掩码展示 + 凭据 ID（编辑弹窗引用该条做受控明文拉取） */
export interface ProviderInlineKeyItem {
  ID: number;
  /** 服务端掩码（短 key/解密失败退化 ****）；详情只出掩码，明文走 raw 端点显式拉取 */
  KeyMasked: string;
}

/** 凭据分组（详情路径；Source/InlineKeys/PoolID 为派生字段） */
export interface ProviderGroupDetail {
  ID: number;
  ProviderID: number;
  Name: string;
  Weight: number;
  Models: string;
  /** 凭据来源二选一：inline / pool */
  Source: string;
  /** 内联凭据掩码项（仅 inline 来源非空；明文不受控返回） */
  InlineKeys: ProviderInlineKeyItem[];
  /** 关联号池 ID；pool 来源非空，inline 来源为 null（三态禁 omitempty） */
  PoolID: number | null;
}

/** GET /providers/:id 详情：Provider + 展开的 endpoints/groups（编辑弹窗回填用） */
export interface ProviderDetail extends Provider {
  Endpoints: ProviderEndpoint[];
  Groups: ProviderGroupDetail[];
}

/** 协议端点写入体（表单提交/回填用，snake_case 对齐后端 EndpointInput） */
export interface ProviderEndpointInput {
  protocol: string;
  url: string;
  enabled: boolean;
}

/** 内联凭据写入项：ID>0 引用既有凭据（key 空 = 原样保留；key 非空 = 替换该条），
 *  ID=0 表示以 key 命名新增。掩码回填的条目提交时 key 传空串由 ID 保留。 */
export interface ProviderInlineKeyInput {
  id: number;
  key: string;
}

/** 凭据分组写入体（后端 GroupInput；凭据来源二选一：inline_keys / pool_id） */
export interface ProviderGroupInput {
  name: string;
  weight: number;
  models: string;
  source: "inline" | "pool";
  inline_keys: ProviderInlineKeyInput[];
  pool_id: number | null;
}

export interface ProviderTemplate {
  type: string;
  template: string;
}

export interface ProviderModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}

/** GET /providers/model-catalog 单供应商组织级目录（PascalCase 与后端
 *  service/modelsync.ProviderModelCatalog 同源）：Upstream = 分组白名单并集（同步真相），
 *  Custom = config.custom_models（手填）。空值显式空数组（非 null）。 */
export interface ProviderModelCatalog {
  ProviderID: number;
  Upstream: string[];
  Custom: string[];
}

export interface ProviderModelTestResult {
  error?: string;
  message?: string;
  [key: string]: unknown;
}

/** Provider 写请求体。结构化 children（protocols/endpoints/groups）可选：表单提交全量携带；
 *  开关类动作只发顶层标量，走后端 partial-update（children 键缺失 = 仅改标量，不动子表）。 */
export interface ProviderWritePayload {
  name?: string;
  type?: string;
  config?: string;
  console?: string;
  proxy?: string;
  model_endpoint?: boolean;
  model_filter_enabled?: boolean;
  blacklisted?: boolean;
  auth_type?: string;
  protocols?: string[];
  endpoints?: ProviderEndpointInput[];
  groups?: ProviderGroupInput[];
}

export async function getProviders(filters: { name?: string; type?: string } = {}): Promise<Provider[]> {
  const params = new URLSearchParams();
  if (filters.name) {
    params.append("name", filters.name);
  }
  if (filters.type) {
    params.append("type", filters.type);
  }

  const queryString = params.toString();
  const endpoint = queryString ? `/providers?${queryString}` : "/providers";
  return apiRequest<Provider[]>(endpoint);
}

export async function getProvider(id: number): Promise<ProviderDetail> {
  return apiRequest<ProviderDetail>(`/providers/${id}`);
}

/** 受控明文端点返回项：按凭据 id 归位，前端替换掩码行时不依赖数组顺序 */
export interface ProviderInlineKeyRawItem {
  id: number;
  key: string;
}

/** 内联组明文拉取（受控端点）：「显示」动作才触发；按 id 归位（删掩码行未保存也能显示剩余）。
 *  `cache: "no-store"` 与服务端同名响应头构成纵深防御：明文不得进浏览器 HTTP 缓存——
 *  前端 state 清得再干净，缓存文件里的响应体仍然在。 */
export async function getProviderGroupInlineKeysRaw(
  providerId: number,
  groupId: number,
): Promise<{ keys: ProviderInlineKeyRawItem[] }> {
  return apiRequest<{ keys: ProviderInlineKeyRawItem[] }>(
    `/providers/${providerId}/groups/${groupId}/inline-keys/raw`,
    { cache: "no-store" },
  );
}

export async function createProvider(provider: ProviderWritePayload): Promise<ProviderDetail> {
  return apiRequest<ProviderDetail>("/providers", {
    method: "POST",
    body: JSON.stringify(provider),
  });
}

export async function updateProvider(id: number, provider: ProviderWritePayload): Promise<ProviderDetail> {
  return apiRequest<ProviderDetail>(`/providers/${id}`, {
    method: "PUT",
    body: JSON.stringify(provider),
  });
}

export async function deleteProvider(id: number): Promise<void> {
  await apiRequest<void>(`/providers/${id}`, {
    method: "DELETE",
  });
}

export async function getProviderTemplates(): Promise<ProviderTemplate[]> {
  return apiRequest<ProviderTemplate[]>("/providers/template");
}

/** 组织级模型目录聚合（分组白名单并集 + custom_models），前端目录页的数据源，
 *  替代 Config.upstream_models 客户端解析。 */
export async function getProviderModelCatalog(): Promise<ProviderModelCatalog[]> {
  return apiRequest<ProviderModelCatalog[]>("/providers/model-catalog");
}

export async function getProviderModels(
  providerId: number,
  options: { source?: "upstream" | "all" } = {}
): Promise<ProviderModel[]> {
  const params = new URLSearchParams();
  if (options.source) {
    params.append("source", options.source);
  }

  const queryString = params.toString();
  const endpoint = queryString
    ? `/providers/models/${providerId}?${queryString}`
    : `/providers/models/${providerId}`;
  return apiRequest<ProviderModel[]>(endpoint);
}

export async function testProviderModel(providerId: number, model: string): Promise<ProviderModelTestResult> {
  return apiRequest<ProviderModelTestResult>(`/providers/${providerId}/test`, {
    method: "POST",
    body: JSON.stringify({ model }),
  });
}

export async function clearProviderAssociations(
  providerId: number
): Promise<{ provider_id: number; provider_name: string; deleted: number }> {
  return apiRequest<{ provider_id: number; provider_name: string; deleted: number }>(
    `/providers/${providerId}/associations`,
    {
      method: "DELETE",
    }
  );
}
