/** 一批「逐项跑」任务的单项结果。
 *
 *  cancelled 与 failed 必须分开：用户主动中止的项不是「这个模型不可用」的证据，
 *  混进失败计数会让「选择失败的模型」之类的下游动作把被取消的项也算进去。
 */
export type BatchItemOutcome = "success" | "failed" | "cancelled";

type BatchItemDoneResult<T> = {
  item: T;
  outcome: BatchItemOutcome;
  completed: number;
  successCount: number;
  failedCount: number;
  cancelledCount: number;
  testing: number;
};

type RunConcurrentBatchParams<T> = {
  items: T[];
  signal: AbortSignal;
  /** 跑单项。实现方**必须**把 signal 透传到真正的请求上——只检查 signal.aborted 就返回
   *  的话，「取消」只停在调度层：不再派新项，但在飞的那一项会照跑完、照计费。
   *  中止导致的异常自行判为 "cancelled" 返回（可复用 `isAbortError`）。 */
  runItem: (item: T, signal: AbortSignal) => Promise<BatchItemOutcome>;
  concurrency?: number;
  onQueueChange?: (testing: number) => void;
  onItemDone?: (result: BatchItemDoneResult<T>) => void;
};

export type BatchSummary = {
  success: number;
  failed: number;
  cancelled: number;
  /** 整批是否被中止。为 true 时未入队的项一件都没跑，调用方不应报「全部完成」。 */
  aborted: boolean;
};

/** 并发跑一批项：最多 concurrency 个在飞，从队列顺序取件，支持中途取消。
 *
 *  由工作协程自然驱动（取一件、跑完、再取一件），不用轮询等待——轮询会让进度更新
 *  一顿一顿地跳。取消后不再取新项；已在飞的项由 runItem 自己响应 signal 终止。 */
export async function runConcurrentBatch<T>({
  items,
  signal,
  runItem,
  concurrency = 3,
  onQueueChange,
  onItemDone,
}: RunConcurrentBatchParams<T>): Promise<BatchSummary> {
  if (items.length === 0) {
    return { success: 0, failed: 0, cancelled: 0, aborted: signal.aborted };
  }

  let cursor = 0;
  let testing = 0;
  let completed = 0;
  let successCount = 0;
  let failedCount = 0;
  let cancelledCount = 0;

  const nextItem = (): T | null => {
    if (signal.aborted || cursor >= items.length) {
      return null;
    }
    const item = items[cursor];
    cursor += 1;
    return item;
  };

  const worker = async () => {
    while (true) {
      const item = nextItem();
      if (item === null) {
        return;
      }

      testing += 1;
      onQueueChange?.(testing);

      // runItem 自己吞掉中止异常并回 "cancelled"；这里兜住实现方漏网的抛错，
      // 一件崩掉不能让整批 Promise.all 提前 reject。
      let outcome: BatchItemOutcome = "failed";
      try {
        outcome = await runItem(item, signal);
      } catch {
        outcome = "failed";
      }

      if (outcome === "success") {
        successCount += 1;
      } else if (outcome === "cancelled") {
        cancelledCount += 1;
      } else {
        failedCount += 1;
      }
      completed += 1;
      testing -= 1;

      onQueueChange?.(testing);
      onItemDone?.({
        item,
        outcome,
        completed,
        successCount,
        failedCount,
        cancelledCount,
        testing,
      });
    }
  };

  const workerCount = Math.max(1, Math.min(concurrency, items.length));
  await Promise.all(Array.from({ length: workerCount }, () => worker()));

  return {
    success: successCount,
    failed: failedCount,
    cancelled: cancelledCount,
    aborted: signal.aborted,
  };
}
