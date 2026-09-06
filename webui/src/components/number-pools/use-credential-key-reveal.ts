/**
 * 号池凭据明文「显示 / 复制」状态机。
 *
 * 明文来源唯一：受控端点 `GET /pools/:id/credentials/:credId/raw`（列表响应恒为服务端掩码 KeyMasked）。
 *
 * 生命周期契约（安全边界，改动前先读）：
 * - 明文只驻留本 hook 的内存 state，**不进 TanStack Query 缓存**：缓存会把明文长期留在
 *   queryClient 里可被 DevTools 查读，且明文没有 invalidate 语义；
 * - `view`（页码 / 筛选 / 搜索 / 弹窗开关）一变即整体作废，明文不跨视图驻留；
 * - 凭据被删除时调用方**必须**调 `forget(ids)`：被删的 key 往往正因为已泄露/已失效，
 *   它的行虽已从 DOM 消失，明文不能继续躺在 state 里等下次翻页；
 * - 隐藏即删明文（再显示重新拉），不留「藏起来但仍在内存」的中间态；
 * - 复制**不改变**显示态：未显示时静默拉一次 raw 写剪贴板，界面仍显掩码；
 * - 复制一律走 `copySecretWithToast`（禁 prompt 兜底），失败提示引导用户改走「显示 + 手动选中」。
 *
 * 提示文案纪律：成功提示只带 KeyMasked，禁止把明文写进任何用户可见文案（toast / prompt）——
 * 文案会落 DOM 或直接上屏，复述明文等于绕过服务端掩码（契约见 `lib/clipboard.ts`）。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { getCredentialRaw } from "@/lib/api";
import { copySecretWithToast } from "@/lib/clipboard";
import { toErrorMessage } from "@/lib/errors";

/** 在途动作：spinner 必须落在真正被点的那个按钮上；显示与复制共用一把锁（互斥） */
export type CredentialKeyAction = "reveal" | "copy";

type CredentialKeyRevealInput = {
  /** 号池 ID；弹窗关闭时传 null，明文拉取与剪贴板写入一律拒绝 */
  poolId: number | null;
  /** 视图标识（页码/筛选/搜索/开关拼成），变化即作废全部已显示明文 */
  view: string;
};

export type CredentialKeyReveal = {
  /** 该凭据当前显示的明文；未显示为 null（渲染层据此在掩码/明文间切换） */
  plaintextOf: (credId: number) => string | null;
  isRevealed: (credId: number) => boolean;
  /** 在途动作（显示或复制），null = 空闲；渲染层据此转圈并禁用按钮 */
  pendingAction: (credId: number) => CredentialKeyAction | null;
  /** 显示 ↔ 隐藏 */
  toggle: (credId: number) => Promise<void>;
  /** 复制到剪贴板；已显示则复用内存明文，不重复打端点 */
  copy: (credId: number, keyMasked: string) => Promise<void>;
  /** 凭据已不存在（删除）时立即丢弃其明文，不等视图切换 */
  forget: (credIds: number[]) => void;
};

export function useCredentialKeyReveal({
  poolId,
  view,
}: CredentialKeyRevealInput): CredentialKeyReveal {
  const [revealed, setRevealed] = useState<Record<number, string>>({});
  const [pending, setPending] = useState<Readonly<Record<number, CredentialKeyAction>>>({});
  // 单调递增世代号：view/poolId 变化即 +1，在途响应比对不符则整轮丢弃
  // （防翻页后的明文写进新视图，与 providers 详情弹窗的 requestId 守卫同一姿势）
  const generation = useRef(0);

  useEffect(() => {
    generation.current += 1;
    setRevealed({});
    setPending({});
  }, [view, poolId]);

  const markPending = useCallback((credId: number, action: CredentialKeyAction) => {
    setPending((prev) => ({ ...prev, [credId]: action }));
  }, []);

  const dropPending = useCallback((credId: number, startedAt: number) => {
    // 世代已变说明 state 刚被整体重置，此处再写会把旧视图的收尾动作带进新视图——
    // 典型故障：旧请求回来清掉新视图里同一条凭据的 pending，双击守卫随之失效
    if (generation.current !== startedAt) return;
    setPending((prev) => {
      if (prev[credId] == null) return prev;
      const next = { ...prev };
      delete next[credId];
      return next;
    });
  }, []);

  /** 拉明文；世代不符（视图已切换）返回 null 表示「本轮作废」，错误也一并吞掉不再提示 */
  const fetchPlaintext = useCallback(
    async (credId: number, startedAt: number): Promise<string | null> => {
      if (poolId == null) return null;
      try {
        const { key } = await getCredentialRaw(poolId, credId);
        return generation.current === startedAt ? key : null;
      } catch (err) {
        if (generation.current !== startedAt) return null;
        throw err;
      }
    },
    [poolId],
  );

  const toggle = useCallback(
    async (credId: number) => {
      if (poolId == null) return;

      if (revealed[credId] != null) {
        setRevealed((prev) => {
          const next = { ...prev };
          delete next[credId];
          return next;
        });
        return;
      }

      if (pending[credId] != null) return; // 双击守卫：同一条只打一次端点
      const startedAt = generation.current;
      markPending(credId, "reveal");
      try {
        const key = await fetchPlaintext(credId, startedAt);
        if (key == null) return;
        setRevealed((prev) => ({ ...prev, [credId]: key }));
      } catch (err) {
        toast.error(`显示明文失败: ${toErrorMessage(err)}`);
      } finally {
        dropPending(credId, startedAt);
      }
    },
    [poolId, revealed, pending, fetchPlaintext, markPending, dropPending],
  );

  const copy = useCallback(
    async (credId: number, keyMasked: string) => {
      // poolId 检查必须排在缓存分支之前：弹窗已关时残留明文一律不得再出前端
      if (poolId == null) return;

      const cached = revealed[credId];
      if (cached != null) {
        await copySecretWithToast(cached, `已复制 key: ${keyMasked}`);
        return;
      }

      if (pending[credId] != null) return;
      const startedAt = generation.current;
      markPending(credId, "copy");
      try {
        const key = await fetchPlaintext(credId, startedAt);
        if (key == null) return;
        // 复制不改变显示态：明文只交给剪贴板，不写 revealed
        await copySecretWithToast(key, `已复制 key: ${keyMasked}`);
      } catch (err) {
        toast.error(`复制失败: ${toErrorMessage(err)}`);
      } finally {
        dropPending(credId, startedAt);
      }
    },
    [poolId, revealed, pending, fetchPlaintext, markPending, dropPending],
  );

  const forget = useCallback((credIds: number[]) => {
    if (credIds.length === 0) return;
    setRevealed((prev) => {
      // 这批 id 本来就没有明文时原样返回，避免无谓重渲染
      if (!credIds.some((id) => prev[id] != null)) return prev;
      const next = { ...prev };
      credIds.forEach((id) => delete next[id]);
      return next;
    });
  }, []);

  // 引用稳定：单元格组件可据此做 memo，且调用方拿到的是同一份快照而非每渲染新对象
  return useMemo(
    () => ({
      plaintextOf: (credId: number) => revealed[credId] ?? null,
      isRevealed: (credId: number) => revealed[credId] != null,
      pendingAction: (credId: number) => pending[credId] ?? null,
      toggle,
      copy,
      forget,
    }),
    [revealed, pending, toggle, copy, forget],
  );
}
