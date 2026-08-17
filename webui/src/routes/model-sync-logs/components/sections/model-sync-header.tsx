import { FolderSync, RefreshCw } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

type ModelSyncHeaderProps = {
  syncing: boolean;
  onSyncNow: () => void;
};

export function ModelSyncHeader({ syncing, onSyncNow }: ModelSyncHeaderProps) {
  return (
    <PageHeader
      icon={FolderSync}
      title="模型同步日志"
      subtitle="查看上游模型同步记录"
      actions={
        <Button variant="default" size="sm" onClick={onSyncNow} disabled={syncing}>
          <RefreshCw className={syncing ? "animate-spin" : ""} />
          {syncing ? "同步中..." : "立即同步"}
        </Button>
      }
    />
  );
}
