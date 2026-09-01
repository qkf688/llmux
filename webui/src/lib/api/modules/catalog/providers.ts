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

/** 凭据分组（详情路径；Source/InlineKeys/PoolID 为派生字段） */
export interface ProviderGroupDetail {
  ID: number;
  ProviderID: number;
  Name: string;
  Weight: number;
  Models: string;
  /** 凭据来源二选一：inline / pool */
  Source: string;
  /** 内联明文 keys（管理端 JWT 域）；仅 inline 来源、解密成功时非空 */
  InlineKeys: string[];
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

/** 凭据分组写入体（后端 GroupInput；凭据来源二选一：inline_keys / pool_id） */
export interface ProviderGroupInput {
  name: string;
  weight: number;
  models: string;
  source: "inline" | "pool";
  inline_keys: string[];
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
