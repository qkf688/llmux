import type { UpstreamStatus } from "../../types";

interface AllModelsUpstreamStatusProps {
  status: UpstreamStatus;
  upstreamCount: number;
}

export function AllModelsUpstreamStatus({
  status,
  upstreamCount,
}: AllModelsUpstreamStatusProps) {
  if (status === "loading") {
    return (
      <div className="flex items-center gap-2 px-3 py-2 bg-info-tint border border-info/30 rounded-md text-sm text-info-foreground">
        <svg
          className="animate-spin h-4 w-4"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
        正在获取上游模型...
      </div>
    );
  }
  if (status === "success") {
    return (
      <div className="flex items-center gap-2 px-3 py-2 bg-success-tint border border-success/30 rounded-md text-sm text-success-foreground">
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M5 13l4 4L19 7"
          />
        </svg>
        已获取 {upstreamCount} 个上游模型
      </div>
    );
  }
  if (status === "empty") {
    return (
      <div className="flex items-center gap-2 px-3 py-2 bg-warning-tint border border-warning/30 rounded-md text-sm text-warning-foreground">
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
        上游未返回任何模型
      </div>
    );
  }
  if (status === "error") {
    return (
      <div className="flex items-center gap-2 px-3 py-2 bg-destructive-tint border border-destructive/30 rounded-md text-sm text-destructive-tint-foreground">
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
        获取上游模型失败
      </div>
    );
  }
  return null;
}
