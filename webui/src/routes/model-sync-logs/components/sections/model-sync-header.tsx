import { Button } from "@/components/ui/button";
import { RefreshCw } from "lucide-react";

type ModelSyncHeaderProps = {
  syncing: boolean;
  onSyncNow: () => void;
};

export function ModelSyncHeader({ syncing, onSyncNow }: ModelSyncHeaderProps) {
  return (
    <div className="flex items-center justify-between flex-shrink-0">
      <div>
        <h2 className="text-2xl font-bold">模型同步日志</h2>
        <p className="text-sm text-muted-foreground">查看上游模型同步记录</p>
      </div>
      <Button variant="default" size="sm" onClick={onSyncNow} disabled={syncing}>
        {syncing ? (
          <>
            <RefreshCw className="h-4 w-4 mr-1 animate-spin" />
            同步中...
          </>
        ) : (
          <>
            <RefreshCw className="h-4 w-4 mr-1" />
            立即同步
          </>
        )}
      </Button>
    </div>
  );
}
