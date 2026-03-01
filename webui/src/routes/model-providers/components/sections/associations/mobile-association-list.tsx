import type { AssociationBatchTestResult } from "../../../types";
import type { ModelWithProvider, Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { Spinner } from "@/components/ui/spinner";
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
import { CapabilityBadges } from "../../shared/capability-badges";
import { MobileInfoItem } from "../../shared/mobile-info-item";
import { StatusBars } from "../../shared/status-bars";

type MobileAssociationListProps = {
  associations: ModelWithProvider[];
  providers: Provider[];
  selectedAssociationIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  providerStatus: Record<number, boolean[]>;
  healthStatus: Record<number, boolean[]>;
  statusUpdating: Record<number, boolean>;
  associationTestResults: Record<number, AssociationBatchTestResult>;
  deleteId: number | null;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onOpenBatchActionSheet: () => void;
  onToggleStatus: (association: ModelWithProvider, nextStatus: boolean) => void;
  onEdit: (association: ModelWithProvider) => void;
  onOpenDelete: (id: number) => void;
  onDeleteDialogChange: (open: boolean) => void;
  onDeleteConfirm: () => void;
  onTest: (id: number) => void;
};

export function MobileAssociationList({
  associations,
  providers,
  selectedAssociationIds,
  isAllSelected,
  isPartialSelected,
  providerStatus,
  healthStatus,
  statusUpdating,
  associationTestResults,
  deleteId,
  onSelectAll,
  onSelectOne,
  onOpenBatchActionSheet,
  onToggleStatus,
  onEdit,
  onOpenDelete,
  onDeleteDialogChange,
  onDeleteConfirm,
  onTest,
}: MobileAssociationListProps) {
  return (
    <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-3 divide-y divide-border">
      <div className="py-1.5 space-y-1.5 border-b">
        <div className="flex items-center gap-2">
          <Checkbox
            checked={isAllSelected}
            ref={(el) => {
              if (el) {
                (el as unknown as HTMLInputElement).indeterminate = isPartialSelected;
              }
            }}
            onCheckedChange={onSelectAll}
            aria-label="全选"
          />
          <span className="text-sm text-muted-foreground">
            {selectedAssociationIds.length > 0 ? `已选择 ${selectedAssociationIds.length} 项` : "全选"}
          </span>
        </div>
        <Button variant="outline" size="sm" onClick={onOpenBatchActionSheet} className="h-7 text-xs w-full">
          批量操作
          {selectedAssociationIds.length > 0 && ` (${selectedAssociationIds.length})`}
        </Button>
      </div>

      {associations.map((association) => {
        const provider = providers.find((item) => item.ID === association.ProviderID);
        const isAssociationEnabled = association.Status ?? true;
        const statusBars = providerStatus[association.ID];
        const healthBars = healthStatus[association.ID];
        const currentResult = associationTestResults[association.ID];

        return (
          <div key={association.ID} className="py-2 space-y-2">
            <div className="flex items-start justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0 flex-1">
                <Checkbox
                  checked={selectedAssociationIds.includes(association.ID)}
                  onCheckedChange={(checked) => onSelectOne(association.ID, !!checked)}
                  aria-label={`选择 ${association.ProviderModel}`}
                />
                <div className="min-w-0 flex-1">
                  <h3 className="font-semibold text-xs truncate">{provider?.Name ?? "未知提供商"}</h3>
                  <p className="text-[10px] text-muted-foreground">提供商模型: {association.ProviderModel}</p>
                </div>
              </div>
              <span
                className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full ${
                  isAssociationEnabled ? "bg-emerald-100 text-emerald-700" : "bg-red-100 text-red-700"
                }`}
              >
                {isAssociationEnabled ? "已启用" : "已停用"}
              </span>
            </div>

            <div className="grid grid-cols-2 gap-3 text-xs">
              <MobileInfoItem label="提供商类型" value={provider?.Type ?? "未知"} />
              <MobileInfoItem label="提供商 ID" value={<span className="font-mono text-xs">{provider?.ID ?? "-"}</span>} />
              <MobileInfoItem label="权重" value={association.Weight} />
              <MobileInfoItem label="优先级" value={association.Priority ?? 100} />
            </div>

            <div className="space-y-3 text-xs">
              <MobileInfoItem
                label="模型能力"
                value={
                  <CapabilityBadges
                    toolCall={association.ToolCall}
                    structuredOutput={association.StructuredOutput}
                    image={association.Image}
                    withHeader={association.WithHeader}
                  />
                }
              />
              <div className="grid grid-cols-2 gap-3">
                <MobileInfoItem
                  label="最近状态"
                  value={
                    <div className="flex items-center gap-1">
                      <StatusBars
                        bars={statusBars}
                        successClassName="bg-green-500"
                        failClassName="bg-red-500"
                        barClassName="w-1 h-4 rounded"
                        emptyTextClassName="text-muted-foreground text-[11px]"
                      />
                    </div>
                  }
                />
                <MobileInfoItem
                  label="健康检测"
                  value={
                    <div className="flex items-center gap-1">
                      <StatusBars
                        bars={healthBars}
                        successClassName="bg-emerald-500"
                        failClassName="bg-orange-500"
                        barClassName="w-1 h-4 rounded"
                        emptyTextClassName="text-muted-foreground text-[11px]"
                      />
                    </div>
                  }
                />
              </div>
            </div>

            <div className="flex items-center justify-between rounded-md border bg-muted/30 px-2 py-1.5">
              <p className="text-[11px] text-muted-foreground">启用状态</p>
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium">{isAssociationEnabled ? "启用" : "停用"}</span>
                <Switch
                  checked={isAssociationEnabled}
                  disabled={!!statusUpdating[association.ID]}
                  onCheckedChange={(value) => onToggleStatus(association, value)}
                  aria-label="切换启用状态"
                />
              </div>
            </div>

            {currentResult && (
              <div className="rounded-md border bg-muted/30 px-2 py-1.5">
                <p className="text-[11px] text-muted-foreground mb-1">测试结果</p>
                <div className="flex items-center gap-2">
                  {currentResult.loading ? (
                    <>
                      <Spinner className="w-4 h-4" />
                      <span className="text-xs">测试中...</span>
                    </>
                  ) : currentResult.success === true ? (
                    <span className="text-sm text-green-600 font-medium">✓ 测试成功</span>
                  ) : currentResult.success === false ? (
                    <div className="flex-1">
                      <span className="text-sm text-red-600 font-medium">✗ 测试失败</span>
                      {currentResult.error && (
                        <p className="text-xs text-muted-foreground mt-1 break-words">{currentResult.error}</p>
                      )}
                    </div>
                  ) : null}
                </div>
              </div>
            )}

            <div className="flex flex-wrap justify-end gap-1">
              <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => onEdit(association)}>
                编辑
              </Button>
              <AlertDialog open={deleteId === association.ID} onOpenChange={(open) => onDeleteDialogChange(open)}>
                <Button
                  variant="destructive"
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => onOpenDelete(association.ID)}
                >
                  删除
                </Button>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>确定要删除这个关联吗？</AlertDialogTitle>
                    <AlertDialogDescription>
                      此操作无法撤销。这将永久删除该模型提供商关联。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel onClick={() => onDeleteDialogChange(false)}>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={onDeleteConfirm}>确认删除</AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
              <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => onTest(association.ID)}>
                测试
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}

