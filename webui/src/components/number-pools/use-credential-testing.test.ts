import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// 测试端点要按请求挂起/放行才能编排「在途遇翻页」竞态，必须在 vi.mock 工厂外持有引用
const { testPoolCredentialMock } = vi.hoisted(() => ({
  testPoolCredentialMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  testPoolCredential: testPoolCredentialMock,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

import { toast } from "sonner";
import { useCredentialTesting } from "./use-credential-testing";

const POOL_ID = 7;
const VIEW = "1|all|";

// 传输错误（HTTP 非 2xx）apiRequest 抛的形状——**不解析响应体**（lib/api/core/client.ts）。
// 失败的测试不是这个：它 HTTP 200，结论在 passed:false（见下方成功/失败用例）。
const HTTP_500 = new Error("API request failed: 500 Internal Server Error");

type TestingProps = { poolId: number | null; view: string };

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function renderTesting(initialProps: TestingProps = { poolId: POOL_ID, view: VIEW }) {
  return renderHook(({ poolId, view }: TestingProps) => useCredentialTesting({ poolId, view }), {
    initialProps,
  });
}

beforeEach(() => {
  testPoolCredentialMock.mockReset();
  vi.mocked(toast.error).mockClear();
});

describe("useCredentialTesting 结论", () => {
  it("passed:true → 成功态（✓），带命中的分组/模型回显，不打 toast", async () => {
    testPoolCredentialMock.mockResolvedValue({ passed: true, group_id: 3, group_name: "g", model: "gpt-x" });
    const { result } = renderTesting();

    await act(async () => {
      await result.current.run(11);
    });

    expect(testPoolCredentialMock).toHaveBeenCalledWith(POOL_ID, 11);
    // #24-B 回显：outcome 带后端实际命中的分组名与测试模型
    expect(result.current.outcomeOf(11)).toEqual({ loading: false, success: true, group: "g", model: "gpt-x" });
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("失败测试（HTTP 200 + passed:false）→ ✕ 且带 error 文本，不走异常路径", async () => {
    testPoolCredentialMock.mockResolvedValue({ passed: false, error_type: "auth", error: "status code: 401" });
    const { result } = renderTesting();

    await act(async () => {
      await result.current.run(11);
    });

    expect(result.current.outcomeOf(11)).toEqual({
      loading: false,
      success: false,
      error: "status code: 401",
    });
    // 结论来自 data.passed，不是抛异常 → 不弹传输错误 toast
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("传输错误（非 2xx throw）→ ✕ 且弹测试失败 toast，不卡在 loading", async () => {
    testPoolCredentialMock.mockRejectedValue(HTTP_500);
    const { result } = renderTesting();

    await act(async () => {
      await result.current.run(11);
    });

    expect(result.current.outcomeOf(11)?.success).toBe(false);
    expect(String(vi.mocked(toast.error).mock.calls[0]?.[0])).toContain("测试失败");
  });

  it("在途时重复点不重复打端点（双击守卫）", async () => {
    const deferred = createDeferred<{ passed: boolean }>();
    testPoolCredentialMock.mockReturnValue(deferred.promise);
    const { result } = renderTesting();

    await act(async () => {
      void result.current.run(11);
    });
    expect(result.current.outcomeOf(11)?.loading).toBe(true);

    await act(async () => {
      void result.current.run(11);
    });

    await act(async () => {
      deferred.resolve({ passed: true });
    });

    expect(testPoolCredentialMock).toHaveBeenCalledTimes(1);
    expect(result.current.outcomeOf(11)).toEqual({ loading: false, success: true });
  });
});

describe("useCredentialTesting 生命周期", () => {
  it("视图切换即作废全部历史结论", async () => {
    testPoolCredentialMock.mockResolvedValue({ passed: true });
    const { result, rerender } = renderTesting();
    await act(async () => {
      await result.current.run(11);
    });

    rerender({ poolId: POOL_ID, view: "2|all|" });

    expect(result.current.outcomeOf(11)).toBeUndefined();
  });

  it("在途结论遇视图切换被丢弃，不写进新视图（世代守卫）", async () => {
    const deferred = createDeferred<{ passed: boolean }>();
    testPoolCredentialMock.mockReturnValue(deferred.promise);
    const { result, rerender } = renderTesting();
    await act(async () => {
      void result.current.run(11);
    });

    rerender({ poolId: POOL_ID, view: "2|all|" });
    await act(async () => {
      deferred.resolve({ passed: true });
    });

    expect(result.current.outcomeOf(11)).toBeUndefined();
  });

  it("forget 只丢弃被删凭据的结论，同页其它行不受影响", async () => {
    testPoolCredentialMock.mockResolvedValue({ passed: true });
    const { result } = renderTesting();
    await act(async () => {
      await result.current.run(11);
    });
    await act(async () => {
      await result.current.run(12);
    });

    act(() => {
      result.current.forget([11]);
    });

    expect(result.current.outcomeOf(11)).toBeUndefined();
    expect(result.current.outcomeOf(12)?.success).toBe(true);
  });

  it("poolId 为 null（弹窗关闭）时拒绝任何测试请求", async () => {
    testPoolCredentialMock.mockResolvedValue({ passed: true });
    const { result, rerender } = renderTesting();

    rerender({ poolId: null, view: "closed" });
    await act(async () => {
      await result.current.run(11);
    });

    expect(testPoolCredentialMock).not.toHaveBeenCalled();
  });
});
