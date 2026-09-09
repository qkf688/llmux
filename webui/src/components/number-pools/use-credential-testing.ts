/**
 * 号池凭据「按凭据测试」结果状态机（#24）。
 *
 * 与 `useCredentialKeyReveal` 同族：测试结论是一次性诊断快照，不是实体数据，
 * **不进 TanStack Query 缓存**（缓存会把它长期留在 queryClient，且无 invalidate 语义）。
 *
 * 生命周期契约：
 * - `view`（页码 / 筛选 / 搜索 / 弹窗开关）一变即整体作废，旧结论不跨视图残留——
 *   否则翻到下一页时上一池/上一页的 ✓/✕ 会错误地显示在新视图同 ID 行上；
 * - 世代号守卫在途响应：请求发出后若视图已切换，回包整轮丢弃（防把过期结论写进新视图）；
 * - 结果按 credId 索引，双击守卫＝同一条 loading 中不可再点。
 *
 * 后端契约：失败的测试也是 HTTP 200，结论在 `passed`；传输/参数错才 throw（进 catch 记 ✕）。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { testPoolCredential } from "@/lib/api";
import type { PoolCredentialTestResult } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";

/** 单条凭据的三态：loading / 成功 / 失败（success===false 时 error 带人读文本）。
 * group/model 是后端实际命中的分组名与测试模型（#24-B 回显：多引用链路时让管理员
 * 看得见测的是哪条链路的哪个模型；链路未选定/未配置时为空，按缺省不展示）。 */
export type CredentialTestOutcome = {
  loading: boolean;
  success: boolean | null;
  error?: string;
  group?: string;
  model?: string;
};

type CredentialTestingInput = {
  poolId: number | null;
  /** 视图标识（页码/筛选/搜索/开关拼成），变化即作废全部历史结论 */
  view: string;
};

export type CredentialTesting = {
  outcomeOf: (credId: number) => CredentialTestOutcome | undefined;
  /** 测某条凭据；loading 中重复点为 no-op（双击守卫） */
  run: (credId: number) => Promise<void>;
  /** 凭据已不存在（删除）时立即丢弃其结论 */
  forget: (credIds: number[]) => void;
};

function outcomeFromResult(result: PoolCredentialTestResult): CredentialTestOutcome {
  const group = result?.group_name || undefined;
  const model = result?.model || undefined;
  if (result?.passed === true) return { loading: false, success: true, group, model };
  return { loading: false, success: false, error: result?.error || "测试未通过", group, model };
}

export function useCredentialTesting({ poolId, view }: CredentialTestingInput): CredentialTesting {
  const [outcomes, setOutcomes] = useState<Record<number, CredentialTestOutcome>>({});
  const generation = useRef(0);

  useEffect(() => {
    generation.current += 1;
    setOutcomes({});
  }, [view, poolId]);

  const run = useCallback(
    async (credId: number) => {
      if (poolId == null) return;
      // 双击守卫：同一条在途时忽略
      if (outcomes[credId]?.loading) return;

      const startedAt = generation.current;
      setOutcomes((prev) => ({ ...prev, [credId]: { loading: true, success: null } }));
      try {
        const result = await testPoolCredential(poolId, credId);
        if (generation.current !== startedAt) return; // 视图已切换，丢弃过期结论
        setOutcomes((prev) => ({ ...prev, [credId]: outcomeFromResult(result) }));
      } catch (err) {
        if (generation.current !== startedAt) return;
        const message = toErrorMessage(err);
        setOutcomes((prev) => ({ ...prev, [credId]: { loading: false, success: false, error: message } }));
        toast.error(`测试失败: ${message}`);
      }
    },
    [poolId, outcomes],
  );

  const forget = useCallback((credIds: number[]) => {
    if (credIds.length === 0) return;
    setOutcomes((prev) => {
      if (!credIds.some((id) => prev[id] != null)) return prev;
      const next = { ...prev };
      credIds.forEach((id) => delete next[id]);
      return next;
    });
  }, []);

  return useMemo(
    () => ({
      outcomeOf: (credId: number) => outcomes[credId],
      run,
      forget,
    }),
    [outcomes, run, forget],
  );
}
