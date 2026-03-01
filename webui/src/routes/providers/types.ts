export type ModelTestResult = {
  loading: boolean;
  success: boolean | null;
  error?: string;
};

export type BatchTestProgress = {
  total: number;
  completed: number;
  success: number;
  failed: number;
  testing: number;
};

export type UpstreamStatus = "loading" | "success" | "empty" | "error" | "disabled";

export const DEFAULT_BATCH_TEST_PROGRESS: BatchTestProgress = {
  total: 0,
  completed: 0,
  success: 0,
  failed: 0,
  testing: 0,
};
