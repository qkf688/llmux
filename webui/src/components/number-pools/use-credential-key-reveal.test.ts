import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// raw 端点要按请求挂起/放行才能编排「在途遇翻页」竞态，必须在 vi.mock 工厂外持有引用
const { getCredentialRawMock, copySecretWithToastMock } = vi.hoisted(() => ({
  getCredentialRawMock: vi.fn(),
  copySecretWithToastMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  getCredentialRaw: getCredentialRawMock,
}));

// 复制的三级降级提示由 lib/clipboard 承载，此处只关心 hook 有没有把明文与掩码交给它
vi.mock("@/lib/clipboard", () => ({
  copySecretWithToast: copySecretWithToastMock,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

import { toast } from "sonner";
import { useCredentialKeyReveal } from "./use-credential-key-reveal";

const POOL_ID = 7;
const MASKED = "sk-****ab12";
const VIEW = "1|all|";

// apiRequest 在 HTTP 非 2xx 时抛的是 "API request failed: <status> <statusText>"，**不解析响应体**
// （lib/api/core/client.ts）——后端 raw 解密失败写的 "Failed to decrypt key" 到不了前端。
// mock 必须用生产真实形状，否则断言的是一条永不出现的文案（假绿）。
const RAW_HTTP_500 = new Error("API request failed: 500 Internal Server Error");

type RevealProps = { poolId: number | null; view: string };

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function renderReveal(initialProps: RevealProps = { poolId: POOL_ID, view: VIEW }) {
  return renderHook(({ poolId, view }: RevealProps) => useCredentialKeyReveal({ poolId, view }), {
    initialProps,
  });
}

/** 点一次「显示」并放行 */
async function revealOnce(
  result: { current: ReturnType<typeof useCredentialKeyReveal> },
  credId: number,
) {
  await act(async () => {
    await result.current.toggle(credId);
  });
}

beforeEach(() => {
  getCredentialRawMock.mockReset();
  copySecretWithToastMock.mockReset();
  copySecretWithToastMock.mockResolvedValue(true);
  vi.mocked(toast.error).mockClear();
});

describe("useCredentialKeyReveal 显示", () => {
  it("toggle 经受控 raw 端点取明文并交给渲染层", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result } = renderReveal();

    await revealOnce(result, 11);

    expect(getCredentialRawMock).toHaveBeenCalledWith(POOL_ID, 11);
    expect(result.current.plaintextOf(11)).toBe("sk-plain-1");
    expect(result.current.isRevealed(11)).toBe(true);
    expect(result.current.pendingAction(11)).toBeNull();
  });

  it("再点一次即隐藏并删掉内存明文，不打端点", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result } = renderReveal();
    await revealOnce(result, 11);

    await revealOnce(result, 11);

    expect(result.current.plaintextOf(11)).toBeNull();
    expect(result.current.isRevealed(11)).toBe(false);
    expect(getCredentialRawMock).toHaveBeenCalledTimes(1);
  });

  it("隐藏后再显示重新拉端点（隐藏即删，不复用旧明文）", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result } = renderReveal();
    await revealOnce(result, 11);
    await revealOnce(result, 11);

    await revealOnce(result, 11);

    // 显示 → 隐藏 → 再显示：两次拉取（隐藏不保留明文，所以再显示必须重新走受控端点）
    expect(getCredentialRawMock).toHaveBeenCalledTimes(2);
    expect(result.current.isRevealed(11)).toBe(true);
  });

  it("明文在途时二次点击不重复打端点（双击守卫）", async () => {
    const deferred = createDeferred<{ key: string }>();
    getCredentialRawMock.mockReturnValue(deferred.promise);
    const { result } = renderReveal();

    await act(async () => {
      void result.current.toggle(11);
    });
    expect(result.current.pendingAction(11)).toBe("reveal");

    await act(async () => {
      void result.current.toggle(11);
    });

    await act(async () => {
      deferred.resolve({ key: "sk-plain-1" });
    });

    expect(getCredentialRawMock).toHaveBeenCalledTimes(1);
    expect(result.current.plaintextOf(11)).toBe("sk-plain-1");
    expect(result.current.pendingAction(11)).toBeNull();
  });

  it("端点失败时提示原因且不卡在 pending", async () => {
    getCredentialRawMock.mockRejectedValue(RAW_HTTP_500);
    const { result } = renderReveal();

    await act(async () => {
      await result.current.toggle(11);
    });

    const message = String(vi.mocked(toast.error).mock.calls[0]?.[0]);
    expect(message).toContain("显示明文失败");
    expect(message).toContain("500");
    expect(result.current.isRevealed(11)).toBe(false);
    expect(result.current.pendingAction(11)).toBeNull();
  });
});

describe("useCredentialKeyReveal 复制", () => {
  it("未显示时静默拉明文写剪贴板：提示只带掩码，且不改显示态", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result } = renderReveal();

    await act(async () => {
      await result.current.copy(11, MASKED);
    });

    expect(copySecretWithToastMock).toHaveBeenCalledWith("sk-plain-1", `已复制 key: ${MASKED}`);
    expect(result.current.isRevealed(11)).toBe(false);
    expect(result.current.plaintextOf(11)).toBeNull();
  });

  it("已显示时复用内存明文，不重复打端点", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result } = renderReveal();
    await revealOnce(result, 11);

    await act(async () => {
      await result.current.copy(11, MASKED);
    });

    expect(getCredentialRawMock).toHaveBeenCalledTimes(1);
    expect(copySecretWithToastMock).toHaveBeenCalledWith("sk-plain-1", `已复制 key: ${MASKED}`);
  });

  it("复制在途时 pending 标记为 copy（spinner 落在复制按钮而非眼睛上）", async () => {
    const deferred = createDeferred<{ key: string }>();
    getCredentialRawMock.mockReturnValue(deferred.promise);
    const { result } = renderReveal();

    await act(async () => {
      void result.current.copy(11, MASKED);
    });
    expect(result.current.pendingAction(11)).toBe("copy");

    await act(async () => {
      deferred.resolve({ key: "sk-plain-1" });
    });

    expect(result.current.pendingAction(11)).toBeNull();
    expect(result.current.isRevealed(11)).toBe(false);
  });

  it("取明文失败时提示复制失败，不写剪贴板", async () => {
    getCredentialRawMock.mockRejectedValue(RAW_HTTP_500);
    const { result } = renderReveal();

    await act(async () => {
      await result.current.copy(11, MASKED);
    });

    expect(copySecretWithToastMock).not.toHaveBeenCalled();
    expect(String(vi.mocked(toast.error).mock.calls[0]?.[0])).toContain("复制失败");
    expect(result.current.pendingAction(11)).toBeNull();
  });
});

describe("useCredentialKeyReveal 明文生命周期", () => {
  it("视图切换（翻页/筛选/关弹窗）即作废全部明文", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result, rerender } = renderReveal();
    await revealOnce(result, 11);

    rerender({ poolId: POOL_ID, view: "2|all|" });

    expect(result.current.plaintextOf(11)).toBeNull();
    expect(result.current.isRevealed(11)).toBe(false);
  });

  it("在途明文遇视图切换被丢弃，不写进新视图", async () => {
    const deferred = createDeferred<{ key: string }>();
    getCredentialRawMock.mockReturnValue(deferred.promise);
    const { result, rerender } = renderReveal();
    await act(async () => {
      void result.current.toggle(11);
    });

    rerender({ poolId: POOL_ID, view: "2|all|" });
    await act(async () => {
      deferred.resolve({ key: "sk-stale" });
    });

    expect(result.current.plaintextOf(11)).toBeNull();
    expect(result.current.pendingAction(11)).toBeNull();
  });

  it("旧视图的晚到响应不得清掉新视图同一条凭据的 pending（dropPending 世代守卫）", async () => {
    const first = createDeferred<{ key: string }>();
    const second = createDeferred<{ key: string }>();
    getCredentialRawMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const { result, rerender } = renderReveal();

    await act(async () => {
      void result.current.toggle(11);
    });
    rerender({ poolId: POOL_ID, view: "2|all|" });
    await act(async () => {
      void result.current.toggle(11);
    });

    expect(getCredentialRawMock).toHaveBeenCalledTimes(2);
    expect(result.current.pendingAction(11)).toBe("reveal");

    // 旧世代响应晚到：不得动新视图的 pending（否则双击守卫失效），也不得污染明文
    await act(async () => {
      first.resolve({ key: "sk-stale" });
    });

    expect(result.current.pendingAction(11)).toBe("reveal");
    expect(result.current.plaintextOf(11)).toBeNull();

    await act(async () => {
      second.resolve({ key: "sk-fresh" });
    });

    expect(result.current.plaintextOf(11)).toBe("sk-fresh");
    expect(result.current.pendingAction(11)).toBeNull();
  });

  it("forget 只丢弃被删凭据的明文，同页其它行不受影响", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result } = renderReveal();
    await revealOnce(result, 11);
    await revealOnce(result, 12);

    act(() => {
      result.current.forget([11]);
    });

    expect(result.current.plaintextOf(11)).toBeNull();
    expect(result.current.plaintextOf(12)).toBe("sk-plain-1");
  });

  it("poolId 为 null（弹窗关闭）时拒绝任何明文拉取与剪贴板写入", async () => {
    getCredentialRawMock.mockResolvedValue({ key: "sk-plain-1" });
    const { result, rerender } = renderReveal();
    await revealOnce(result, 11);

    rerender({ poolId: null, view: "closed" });
    await act(async () => {
      await result.current.toggle(11);
      await result.current.copy(11, MASKED);
    });

    // 关闭前只打过一次端点；关闭后 toggle/copy 一律拒绝（含 poolId 检查前置于缓存分支）
    expect(getCredentialRawMock).toHaveBeenCalledTimes(1);
    expect(copySecretWithToastMock).not.toHaveBeenCalled();
  });
});
