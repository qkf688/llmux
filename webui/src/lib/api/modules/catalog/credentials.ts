/**
 * 凭据 API（`/api/credentials…`）：凭据是第一等资源，归属号池只是它的字段与列表过滤条件——
 * 号池页与供应商编辑弹窗共用本模块。
 *
 * 为什么从 pools 模块拆出来：原先凭据的读/写挂在 `/api/pools/:id/credentials…`，
 * 与号池强绑定，分组拿不到按凭据 ID 的属性与动作（判停后无法人工恢复）。
 * 提为独立资源后，启停/备注/明文/以后的凭据级设置只有这一份实现。
 */
import { apiRequest } from "../../core/client";

/** 凭据状态机（与 models.CredentialStatus 对齐；冷却用 CooldownUntil 表达，不占状态） */
export type CredentialStatus = "active" | "disabled" | "error" | "temp_unsched";

/** 凭据视图（与后端 service/credview.View 同源，PascalCase）：号池列表与供应商分组
 *  详情共用同一形状。三态字段（PoolID/CooldownUntil/…）后端恒出键，
 *  `null` = 无该归属 / 无冷却，前端据此判断，禁写 `?`。 */
export interface CredentialView {
  ID: number;
  PoolID: number | null;
  Status: CredentialStatus | string;
  Note: string;
  KeyMasked: string;
  CooldownUntil: string | null;
  CooldownReason: string;
  FailCount: number;
  LastUsedAt: string | null;
  LastProbeAt: string | null;
  TotalRequests: number;
  TotalErrors: number;
  TotalTokens: number;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface CredentialListData {
  items: CredentialView[];
  total: number;
  page: number;
  page_size: number;
}

export interface CredentialListParams {
  /** 归属过滤（号池 id；不带走全量） */
  pool_id?: number;
  page?: number;
  page_size?: number;
  status?: string;
  q?: string;
}

export function listCredentials(params: CredentialListParams = {}): Promise<CredentialListData> {
  const sp = new URLSearchParams();
  if (params.pool_id != null) sp.append("pool_id", String(params.pool_id));
  if (params.page != null) sp.append("page", String(params.page));
  if (params.page_size != null) sp.append("page_size", String(params.page_size));
  if (params.status && params.status !== "all") sp.append("status", params.status);
  if (params.q) sp.append("q", params.q);
  const qs = sp.toString();
  return apiRequest<CredentialListData>(qs ? `/credentials?${qs}` : "/credentials");
}

export function getCredential(credId: number): Promise<CredentialView> {
  return apiRequest<CredentialView>(`/credentials/${credId}`);
}

/** 新建凭据：`pool_id` 必填（凭据只归属号池）。 */
export function createCredential(body: {
  pool_id: number;
  key: string;
  note?: string;
}): Promise<CredentialView> {
  return apiRequest<CredentialView>("/credentials", {
    method: "POST",
    body: JSON.stringify({ pool_id: body.pool_id, key: body.key, note: body.note ?? "" }),
  });
}

/** 更新单条凭据：`status` 切 active 即「人工恢复」（后端连带清连败计数与判停原因）。 */
export function updateCredential(
  credId: number,
  body: { note?: string | null; status?: string },
): Promise<CredentialView> {
  return apiRequest<CredentialView>(`/credentials/${credId}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export function deleteCredential(credId: number): Promise<void> {
  return apiRequest<void>(`/credentials/${credId}`, { method: "DELETE" });
}

/** 单条凭据明文拉取（受控端点）：号池详情「显示 / 复制」动作才触发。
 *  `cache: "no-store"` 与服务端同名响应头构成纵深防御：明文不得进浏览器 HTTP 缓存——
 *  前端 state 清得再干净，缓存文件里的响应体仍然在。 */
export function getCredentialRaw(credId: number): Promise<{ key: string }> {
  return apiRequest<{ key: string }>(`/credentials/${credId}/raw`, { cache: "no-store" });
}

export function batchUpdateCredentialStatus(
  body: { ids: number[]; status: string },
): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>("/credentials/status", {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export function batchDeleteCredentials(body: { ids: number[] }): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>("/credentials/batch", {
    method: "DELETE",
    body: JSON.stringify(body),
  });
}