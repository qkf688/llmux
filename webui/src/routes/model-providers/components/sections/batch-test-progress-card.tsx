import type { AssociationBatchTestResult, BatchTestProgress } from "../../types";
import { Button } from "@/components/ui/button";

type BatchTestProgressCardProps = {
  batchTesting: boolean;
  batchTestProgress: BatchTestProgress;
  associationTestResults: Record<number, AssociationBatchTestResult>;
  onCancel: () => void;
  onClear: () => void;
  onSelectSuccess: () => void;
  onSelectFailed: () => void;
};

export function BatchTestProgressCard({
  batchTesting,
  batchTestProgress,
  associationTestResults,
  onCancel,
  onClear,
  onSelectSuccess,
  onSelectFailed,
}: BatchTestProgressCardProps) {
  const hasResults = Object.keys(associationTestResults).length > 0;
  const hasSuccess = Object.values(associationTestResults).some((result) => result.success === true);
  const hasFailed = Object.values(associationTestResults).some((result) => result.success === false);

  if (!batchTesting && batchTestProgress.total === 0) {
    return null;
  }

  return (
    <div className="rounded-md border bg-card p-3 space-y-2">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <h3 className="text-xs font-medium">{batchTesting ? "批量测试进行中" : "批量测试完成"}</h3>
          <p className="text-xs text-muted-foreground">
            进度: {batchTestProgress.completed} / {batchTestProgress.total}
            {batchTesting && batchTestProgress.testing > 0 && ` (正在测试: ${batchTestProgress.testing})`}
          </p>
        </div>
        <div className="flex gap-2">
          {batchTesting ? (
            <Button variant="outline" size="sm" onClick={onCancel} className="h-7 px-2 text-xs">
              取消测试
            </Button>
          ) : (
            <Button variant="outline" size="sm" onClick={onClear} className="h-7 px-2 text-xs">
              清除结果
            </Button>
          )}
        </div>
      </div>

      {batchTesting && (
        <div className="w-full bg-secondary rounded-full h-1.5">
          <div
            className="bg-primary h-1.5 rounded-full transition-all duration-300"
            style={{ width: `${(batchTestProgress.completed / batchTestProgress.total) * 100}%` }}
          />
        </div>
      )}

      <div className="flex gap-3 text-xs">
        <span className="text-success">成功: {batchTestProgress.success}</span>
        <span className="text-destructive">失败: {batchTestProgress.failed}</span>
      </div>

      {!batchTesting && hasResults && (
        <div className="flex items-center gap-2 pt-2 border-t">
          <span className="text-xs text-muted-foreground">快捷选择:</span>
          <div className="flex gap-2 flex-1 justify-end">
            <Button
              variant="outline"
              size="sm"
              onClick={onSelectSuccess}
              disabled={!hasSuccess}
              className="h-7 text-xs"
            >
              选择成功项
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={onSelectFailed}
              disabled={!hasFailed}
              className="h-7 text-xs"
            >
              选择失败项
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
