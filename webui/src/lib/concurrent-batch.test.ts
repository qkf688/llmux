import { describe, expect, it } from "vitest";

import { runConcurrentBatch } from "./concurrent-batch";

describe("runConcurrentBatch", () => {
  it("cancelled 单独计数，不混入 failed", async () => {
    const controller = new AbortController();
    const outcomes: Record<number, "success" | "failed" | "cancelled"> = {
      1: "success",
      2: "failed",
      3: "cancelled",
    };

    const summary = await runConcurrentBatch({
      items: [1, 2, 3],
      signal: controller.signal,
      concurrency: 1,
      runItem: async (item) => outcomes[item],
    });

    expect(summary).toEqual({ success: 1, failed: 1, cancelled: 1, aborted: false });
  });

  it("中止后不再取新项，未入队的项不产生结果", async () => {
    const controller = new AbortController();
    const seen: number[] = [];

    const summary = await runConcurrentBatch({
      items: [1, 2, 3, 4],
      signal: controller.signal,
      concurrency: 1,
      runItem: async (item) => {
        seen.push(item);
        if (item === 1) {
          controller.abort();
        }
        return "cancelled";
      },
    });

    // 中止发生在第一项执行中：其余三项不该被取用，也不该出现在计数里。
    expect(seen).toEqual([1]);
    expect(summary).toEqual({ success: 0, failed: 0, cancelled: 1, aborted: true });
  });

  it("runItem 抛错兜为 failed，不 reject 整批", async () => {
    const controller = new AbortController();

    const summary = await runConcurrentBatch({
      items: [1],
      signal: controller.signal,
      runItem: async () => {
        throw new Error("boom");
      },
    });

    expect(summary).toEqual({ success: 0, failed: 1, cancelled: 0, aborted: false });
  });

  it("空列表直接返回，不启动协程", async () => {
    const controller = new AbortController();
    let called = false;

    const summary = await runConcurrentBatch({
      items: [],
      signal: controller.signal,
      runItem: async () => {
        called = true;
        return "success";
      },
    });

    expect(called).toBe(false);
    expect(summary).toEqual({ success: 0, failed: 0, cancelled: 0, aborted: false });
  });
});
