type BatchItemDoneResult<T> = {
  item: T;
  success: boolean;
  completed: number;
  successCount: number;
  failedCount: number;
  testing: number;
};

type RunConcurrentBatchParams<T> = {
  items: T[];
  signal: AbortSignal;
  runItem: (item: T, signal: AbortSignal) => Promise<boolean>;
  concurrency?: number;
  onQueueChange?: (testing: number) => void;
  onItemDone?: (result: BatchItemDoneResult<T>) => void;
};

export async function runConcurrentBatch<T>({
  items,
  signal,
  runItem,
  concurrency = 3,
  onQueueChange,
  onItemDone,
}: RunConcurrentBatchParams<T>): Promise<{ success: number; failed: number; aborted: boolean }> {
  if (items.length === 0) {
    return { success: 0, failed: 0, aborted: signal.aborted };
  }

  let cursor = 0;
  let testing = 0;
  let completed = 0;
  let successCount = 0;
  let failedCount = 0;

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

      let succeeded = false;
      try {
        succeeded = await runItem(item, signal);
      } catch {
        succeeded = false;
      }

      if (succeeded) {
        successCount += 1;
      } else {
        failedCount += 1;
      }
      completed += 1;
      testing -= 1;

      onQueueChange?.(testing);
      onItemDone?.({
        item,
        success: succeeded,
        completed,
        successCount,
        failedCount,
        testing,
      });
    }
  };

  const workerCount = Math.max(1, Math.min(concurrency, items.length));
  await Promise.all(Array.from({ length: workerCount }, () => worker()));

  return {
    success: successCount,
    failed: failedCount,
    aborted: signal.aborted,
  };
}
