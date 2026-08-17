import { useState } from "react";
import { Cloud, RefreshCw } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface ProvidersHeaderProps {
  syncingAll: boolean;
  onSyncAllProviders: () => void;
  /** 当前列表中的供应商数量（筛选后），用于页头副标题 */
  providerCount: number;
  onCreateProvider: () => void;
}

/**
 * providers 页头：标题栏 + 全局操作按钮。
 * 一键同步的确认弹窗与触发它的按钮同属本组件，筛选栏职责在 ProvidersToolbar（SRP）。
 */
export function ProvidersHeader({
  syncingAll,
  onSyncAllProviders,
  providerCount,
  onCreateProvider,
}: ProvidersHeaderProps) {
  const [syncConfirmOpen, setSyncConfirmOpen] = useState(false);

  const handleConfirmSync = () => {
    setSyncConfirmOpen(false);
    onSyncAllProviders();
  };

  return (
    <>
      <PageHeader
        icon={Cloud}
        title="提供商管理"
        subtitle={`当前 ${providerCount} 个上游供应商`}
        actions={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setSyncConfirmOpen(true)}
              disabled={syncingAll}
            >
              <RefreshCw className={syncingAll ? "animate-spin" : ""} />
              {syncingAll ? "同步中..." : "同步上游"}
            </Button>
            <Button size="sm" onClick={onCreateProvider}>
              添加提供商
            </Button>
          </>
        }
      />

      <AlertDialog open={syncConfirmOpen} onOpenChange={setSyncConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认一键同步</AlertDialogTitle>
            <AlertDialogDescription>
              此操作将向所有已配置的提供商发起模型列表同步，可能涉及新增或删除模型记录。确认继续？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmSync}>确认同步</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
