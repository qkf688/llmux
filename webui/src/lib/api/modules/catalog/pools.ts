import type { ErrorType } from "@/lib/errors";
import { apiRequest } from "../../core/client";

export interface Pool {
  ID: number;
  Name: string;
  Note: string;
  /** 池级「测试模型」（#24-B 迁移）：按凭据测试打真实最小 completion 用的模型名；
   *  空 = 未配置，点测直接判 validation 失败。键恒在（后端直返实体无 omitempty）。 */
  TestModel: string;
  /** 该池是否由「供应商分组 → 添加 Key」流程自动创建（专属池）。
   *  号池页据此把自动池折叠成单独一块，与用户手建的多 Key 号池主列表分开。 */
  AutoCreated: boolean;
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

/** 号池写请求体为全量语义（后端 PUT 走 UpdateFields）：test_model 缺省按空串提交 = 清空。 */
export async function createPool(
  body: { name: string; note?: string; test_model?: string },
): Promise<Pool> {
  return apiRequest<Pool>("/pools", {
    method: "POST",
    body: JSON.stringify({ name: body.name, note: body.note ?? "", test_model: body.test_model ?? "" }),
  });
}

export async function updatePool(
  id: number,
  body: { name: string; note?: string; test_model?: string },
): Promise<Pool> {
  return apiRequest<Pool>(`/pools/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name: body.name, note: body.note ?? "", test_model: body.test_model ?? "" }),
  });
}

export async function deletePool(id: number): Promise<void> {
  return apiRequest<void>(`/pools/${id}`, { method: "DELETE" });
}

/** 号池详情「按凭据测试」结果（#24-B，与 testapi ProviderModelTestResult 同族形状）：
 *  一次失败的测试也是 HTTP 200 + code 200，结论在 `passed`（apiRequest 在 code!=200 时
 *  只抛 message、丢 data，故失败不能用异常判定，必须读 data.passed）。
 *  group_id/group_name/provider_id 是后端实际命中的引用链路（取 ProviderID/GroupID 最小
 *  者；0/空串 = 池无引用组、链路未选定）；model 是本次实际使用的测试模型（来自号池
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

export async function batchImportCredentials(
  poolId: number,
  body: { keys: string[] },
): Promise<BatchImportData> {
  return apiRequest<BatchImportData>(`/pools/${poolId}/credentials/batch/import`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** POST /pools/auto/import 响应：自动池标识 + 批量导入逐行回显（与既有批量导入同源）。 */
export interface AutoPoolImportData extends BatchImportData {
  pool_id: number;
  pool_name: string;
}

/**
 * POST /pools/auto/import —— 「供应商分组 → 添加 Key」的一步式落库：
 * 建一个专属号池（AutoCreated=true）并把粘贴的 Key 导进去。
 * name 由调用方按「供应商名·分组名」拼好；后端重名时自动加序号后缀，不会报「池名已存在」。
 */
export async function autoImportPoolCredentials(body: {
  name: string;
  note?: string;
  keys: string[];
}): Promise<AutoPoolImportData> {
  return apiRequest<AutoPoolImportData>("/pools/auto/import", {
    method: "POST",
    body: JSON.stringify({ name: body.name, note: body.note ?? "", keys: body.keys }),
  });
}
