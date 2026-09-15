import { apiRequest, fetchWithAuth } from "../../core/client";

/** 管理端 key 条目，逐键对齐后端 `handler/apikeys` 的 apiKeyResponse
 * （snake_case，AGENTS §5 契约纪律）；api_key 为解密明文（列表端点恒回显，
 * 后端 no-store），单行解密失败时后端降级 "****"，本层按原样透出不特判
 * （消费端 api-keys 卡片列表据 "****" 禁用复制入口）。 */
export interface ApiKeyItem {
  id: number;
  name: string;
  api_key: string;
  enabled: boolean;
  created_at: string;
}

/** 创建提交体（表单/编排层共用单源，#6 加 headers 只改这里）。
 *  value 省略/空 = 后端随机生成；前缀补全与 ≥16 校验单源在后端 NormalizeCallerKeyValue。 */
export type ApiKeyCreateInput = { name: string; value?: string };

/** PATCH 部分更新体：三态语义——未传字段整个键缺席（JSON.stringify 丢 undefined），
 *  后端才不会误改；不提供 value，key 值创建后不可改（轮换 = 新建 + 停用/删除）。 */
export type ApiKeyUpdateInput = { name?: string; enabled?: boolean };

/** GET /api/api-keys 全量列表（含 disabled）。 */
export async function listApiKeys(): Promise<ApiKeyItem[]> {
  return apiRequest<ApiKeyItem[]>("/api-keys");
}

/**
 * 创建 key。不走 apiRequest：hash 撞库是**真实 HTTP 409**（httpresp.Conflict），
 * 而 apiRequest 对非 2xx 抛泛化 Error 并丢弃响应体——冲突原因只在 body 信封里，
 * 丢了用户就只能看到 "Conflict"。故此处经 fetchWithAuth 手动解包，一切失败
 * （409 / 信封 code≠200 / body 非 JSON）统一抛 Error(可展示文案)，UI 只消费 message。
 */
export async function createApiKey(body: ApiKeyCreateInput): Promise<ApiKeyItem> {
  const response = await fetchWithAuth("/api-keys", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: body.name, value: body.value?.trim() || "" }),
  });

  if (!response.ok) {
    let message = `API request failed: ${response.status} ${response.statusText}`;
    try {
      const payload = (await response.json()) as { message?: string };
      if (typeof payload.message === "string" && payload.message.trim()) {
        message = payload.message;
      }
    } catch {
      // body 非 JSON（网关/代理层错误页等）：保留泛化文案，不吞成 undefined
    }
    throw new Error(message);
  }

  let payload: { code: number; message: string; data: ApiKeyItem };
  try {
    payload = (await response.json()) as { code: number; message: string; data: ApiKeyItem };
  } catch {
    // 2xx 但 body 非 JSON（如代理注入 HTML）：SyntaxError 原文不可展示，统一泛化文案
    throw new Error("API request failed: invalid success response body");
  }
  if (payload.code !== 200) {
    throw new Error(payload.message || `API request failed: code ${payload.code}`);
  }
  return payload.data;
}

/** PATCH 部分更新（重命名/启停）。404 走 HTTP 200 + 信封（apiRequest 抛 message
 *  即消费），无 409 面（值不可改）。 */
export async function updateApiKey(id: number, body: ApiKeyUpdateInput): Promise<ApiKeyItem> {
  return apiRequest<ApiKeyItem>(`/api-keys/${id}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

/** DELETE 硬删。在途请求不受影响（/v1 鉴权只在入口执行一次，方案 §2）。 */
export async function deleteApiKey(id: number): Promise<void> {
  return apiRequest<void>(`/api-keys/${id}`, { method: "DELETE" });
}
