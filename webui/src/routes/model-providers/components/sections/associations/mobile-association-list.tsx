import { useState } from "react";
import type { AssociationBatchTestResult } from "../../../types";
import type { ModelWithProvider, Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { ChevronDown, ChevronUp, MoreVertical, TestTube, Trash2 } from "lucide-react";

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
  onToggleStatus,
  onEdit,
  onOpenDelete,
  onDeleteDialogChange,
  onDeleteConfirm,
  onTest,
}: MobileAssociationListProps) {
  const [expandedAssociations, setExpandedAssociations] = useState<Record<number, boolean>>({});

  const toggleExpanded = (id: number) => {
    setExpandedAssociations((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-1 py-2 divide-y divide-border">
      <div className="py-1 border-b">
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
          <span className="text-xs text-muted-foreground">
            {selectedAssociationIds.length > 0 ? `已选择 ${selectedAssociationIds.length} 项` : "全选"}
          </span>
        </div>
      </div>

      {associations.map((association) => {
        const provider = providers.find((item) => item.ID === association.ProviderID);
        const isAssociationEnabled = association.Status ?? true;
        const statusBars = providerStatus[association.ID];
        const healthBars = healthStatus[association.ID];
        const currentResult = associationTestResults[association.ID];
        const isExpanded = !!expandedAssociations[association.ID];

        const testBadge = currentResult?.loading
          ? { text: "测试中", className: "bg-muted text-muted-foreground" }
          : currentResult?.success === true
            ? { text: "成功", className: "bg-emerald-100 text-emerald-700" }
            : currentResult?.success === false
              ? { text: "失败", className: "bg-red-100 text-red-700" }
              : null;

        return (
          <div key={association.ID} className="py-1.5">
            <div className="flex items-start gap-2 px-1">
              <div className="flex items-start gap-2 min-w-0 flex-1">
                <Checkbox
                  checked={selectedAssociationIds.includes(association.ID)}
                  onCheckedChange={(checked) => onSelectOne(association.ID, !!checked)}
                  aria-label={`选择 ${association.ProviderModel}`}
                  className="mt-0.5 shrink-0"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 min-w-0">
                    <h3 className="font-semibold text-[13px] leading-snug truncate">{provider?.Name ?? "未知提供商"}</h3>
                    {testBadge && (
                      <span className={`shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded-full ${testBadge.className}`}>
                        {testBadge.text}
                      </span>
                    )}
                  </div>
                  <p className="text-[10px] text-muted-foreground leading-tight truncate">
                    {association.ProviderModel}
                  </p>
                </div>
              </div>

              <div className="flex items-center gap-1 shrink-0">
                <Switch
                  checked={isAssociationEnabled}
                  disabled={!!statusUpdating[association.ID]}
                  onCheckedChange={(value) => onToggleStatus(association, value)}
                  aria-label="切换启用状态"
                />

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="size-8" aria-label="更多操作">
                      <MoreVertical className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-40">
                    <DropdownMenuItem onClick={() => onEdit(association)} className="cursor-pointer">
                      编辑
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onTest(association.ID)} className="cursor-pointer">
                      <TestTube className="size-4" />
                      测试
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => onOpenDelete(association.ID)}
                      className="cursor-pointer"
                    >
                      <Trash2 className="size-4" />
                      删除
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>

                <Button
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  onClick={() => toggleExpanded(association.ID)}
                  aria-label={isExpanded ? "收起详情" : "展开详情"}
                >
                  {isExpanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                </Button>
              </div>
            </div>

            {isExpanded && (
              <div className="mt-2 space-y-2 ml-6">
                <div className="grid grid-cols-2 gap-x-3 gap-y-2 text-[11px]">
                  <MobileInfoItem label="提供商类型" value={provider?.Type ?? "未知"} />
                  <MobileInfoItem
                    label="提供商 ID"
                    value={<span className="font-mono text-[11px]">{provider?.ID ?? "-"}</span>}
                  />
                  <MobileInfoItem label="权重" value={association.Weight} />
                  <MobileInfoItem label="优先级" value={association.Priority ?? 100} />
                </div>

                <div className="space-y-2">
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
                  <div className="grid grid-cols-2 gap-x-3 gap-y-2">
                    <MobileInfoItem
                      label="最近状态"
                      value={
                        <div className="flex items-center gap-1">
                          <StatusBars
                            bars={statusBars}
                            successClassName="bg-green-500"
                            failClassName="bg-red-500"
                            barClassName="w-1 h-3 rounded"
                            emptyTextClassName="text-muted-foreground text-[10px]"
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
                            barClassName="w-1 h-3 rounded"
                            emptyTextClassName="text-muted-foreground text-[10px]"
                          />
                        </div>
                      }
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

                <div className="text-[11px] text-muted-foreground">
                  当前状态：{isAssociationEnabled ? "启用" : "停用"}
                </div>
              </div>
            )}

            <AlertDialog open={deleteId === association.ID} onOpenChange={(open) => onDeleteDialogChange(open)}>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>确定要删除这个关联吗？</AlertDialogTitle>
                  <AlertDialogDescription>此操作无法撤销。这将永久删除该模型提供商关联。</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel onClick={() => onDeleteDialogChange(false)}>取消</AlertDialogCancel>
                  <AlertDialogAction onClick={onDeleteConfirm}>确认删除</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        );
      })}
    </div>
  );
}
