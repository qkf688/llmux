import type { ErrorType } from "@/lib/errors";
import { apiRequest } from "../../core/client";

/** 凭据状态机（与 models.CredentialStatus 对齐；冷却用 CooldownUntil 表达，不占状态） */
export type CredentialStatus = "active" | "disabled" | "error" | "temp_unsched";

export interface Pool {
  ID: number;
  Name: string;
  Note: string;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: string | null;
}

/** GET /pools 列表项：Pool + 健康概览统计 */
export interface PoolListItem extends Pool {
  KeyCount: number;
  StatusCounts: Record<string, number>;
  ReferencedBy: number;
}

export interface CredentialListItem {
  ID: number;
  PoolID: number | null;
  GroupID: number | null;
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
  items: CredentialListItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface CredentialListParams {
  page?: number;
  page_size?: number;
  status?: string;
  q?: string;
}

export interface BatchImportRow {
  Index: number;
  Key: string;
  Status: "imported" | "skipped" | "failed" | string;
  Reason?: string;
}

export interface BatchImportData {
  total: number;
  imported: number;
  skipped: number;
  failed: number;
  rows: BatchImportRow[];
}

export async function getPools(filters: { name?: string } = {}): Promise<PoolListItem[]> {
  const params = new URLSearchParams();
  if (filters.name) {
    params.append("name", filters.name);
  }
  const qs = params.toString();
  return apiRequest<PoolListItem[]>(qs ? `/pools?${qs}` : "/pools");
}

export async function createPool(body: { name: string; note?: string }): Promise<Pool> {
  return apiRequest<Pool>("/pools", {
    method: "POST",
    body: JSON.stringify({ name: body.name, note: body.note ?? "" }),
  });
}

export async function updatePool(
  id: number,
  body: { name: string; note?: string },
): Promise<Pool> {
  return apiRequest<Pool>(`/pools/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name: body.name, note: body.note ?? "" }),
  });
}

export async function deletePool(id: number): Promise<void> {
  return apiRequest<void>(`/pools/${id}`, { method: "DELETE" });
}

export async function getPoolCredentials(
  poolId: number,
  params: CredentialListParams = {},
): Promise<CredentialListData> {
  const sp = new URLSearchParams();
  if (params.page != null) sp.append("page", String(params.page));
  if (params.page_size != null) sp.append("page_size", String(params.page_size));
  if (params.status && params.status !== "all") sp.append("status", params.status);
  if (params.q) sp.append("q", params.q);
  const qs = sp.toString();
  return apiRequest<CredentialListData>(
    qs ? `/pools/${poolId}/credentials?${qs}` : `/pools/${poolId}/credentials`,
  );
}

export async function createCredential(
  poolId: number,
  body: { key: string; note?: string },
): Promise<CredentialListItem> {
  return apiRequest<CredentialListItem>(`/pools/${poolId}/credentials`, {
    method: "POST",
    body: JSON.stringify({ key: body.key, note: body.note ?? "" }),
  });
}

export async function updateCredential(
  poolId: number,
  credId: number,
  body: { note?: string | null; status?: string },
): Promise<CredentialListItem> {
  return apiRequest<CredentialListItem>(`/pools/${poolId}/credentials/${credId}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export async function deleteCredential(poolId: number, credId: number): Promise<void> {
  return apiRequest<void>(`/pools/${poolId}/credentials/${credId}`, {
    method: "DELETE",
  });
}

/** 单条凭据明文拉取（受控端点）：号池详情「显示 / 复制」动作才触发。
 *  `cache: "no-store"` 与服务端同名响应头构成纵深防御：明文不得进浏览器 HTTP 缓存——
 *  前端 state 清得再干净，缓存文件里的响应体仍然在。 */
export async function getCredentialRaw(
  poolId: number,
  credId: number,
): Promise<{ key: string }> {
  return apiRequest<{ key: string }>(`/pools/${poolId}/credentials/${credId}/raw`, {
    cache: "no-store",
  });
}

/** 号池详情「按凭据测试」结果（#24-B，与 testapi ProviderModelTestResult 同族形状）：
 *  一次失败的测试也是 HTTP 200 + code 200，结论在 `passed`（apiRequest 在 code!=200 时
 *  只抛 message、丢 data，故失败不能用异常判定，必须读 data.passed）。
 *  group_id/group_name/provider_id 是后端实际命中的引用链路（取 ProviderID/GroupID 最小
 *  者；0/空串 = 池无引用组、链路未选定）；model 是本次实际使用的测试模型（来自分组
 *  test_model；空/缺省 = 链路未选定或未配置测试模型）。**只报不改**：本端点不回写凭据状态。 */
export interface PoolCredentialTestResult {
  passed?: boolean;
  error?: string;
  error_type?: ErrorType;
  group_id?: number;
  group_name?: string;
  provider_id?: number;
  model?: string;
  [key: string]: unknown;
}

/** POST /pools/:id/credentials/:credId/test —— 用指定这一条凭据打一次真实最小 completion 探活（#24-B）。 */
export async function testPoolCredential(
  poolId: number,
  credId: number,
): Promise<PoolCredentialTestResult> {
  return apiRequest<PoolCredentialTestResult>(`/pools/${poolId}/credentials/${credId}/test`, {
    method: "POST",
  });
}

export async function batchUpdateCredentialStatus(
  poolId: number,
  body: { ids: number[]; status: string },
): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>(`/pools/${poolId}/credentials/batch/status`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export async function batchDeleteCredentials(
  poolId: number,
  body: { ids: number[] },
): Promise<{ deleted: number }> {
  return apiRequest<{ deleted: number }>(`/pools/${poolId}/credentials/batch`, {
    method: "DELETE",
    body: JSON.stringify(body),
  });
}

export async function batchImportCredentials(
  poolId: number,
  body: { keys: string[] },
): Promise<BatchImportData> {
  return apiRequest<BatchImportData>(`/pools/${poolId}/credentials/batch/import`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}
