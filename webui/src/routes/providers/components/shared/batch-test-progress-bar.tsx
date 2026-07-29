import { Button } from "@/components/ui/button";
import type { BatchTestProgress } from "../../types";

interface BatchTestProgressProps {
  progress: BatchTestProgress;
  onCancel: () => void;
}

export function BatchTestProgressBar({
  progress,
  onCancel,
}: BatchTestProgressProps) {
  const pct = progress.total > 0 ? (progress.completed / progress.total) * 100 : 0;
  return (
    <div className="flex items-center gap-3 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md flex-shrink-0">
      <div className="flex-1">
        <div className="flex items-center justify-between text-xs text-blue-800 mb-1">
          <span>
            测试进度：{progress.completed}/{progress.total}
            (成功: {progress.success}, 失败: {progress.failed}, 进行中:{" "}
            {progress.testing})
          </span>
          <span>{Math.round(pct)}%</span>
        </div>
        <div className="w-full bg-blue-200 rounded-full h-2">
          <div
            className="bg-blue-600 h-2 rounded-full transition-all duration-300"
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>
      <Button
        variant="outline"
        size="sm"
        onClick={onCancel}
        className="h-7 text-xs"
      >
        取消
      </Button>
    </div>
  );
}
