import type { AssociationBatchTestResult } from "../../../types";
import type { ModelWithProvider, Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Spinner } from "@/components/ui/spinner";
import { RefreshCw } from "lucide-react";
import { CapabilityBadges } from "../../shared/capability-badges";
import { StatusBars } from "../../shared/status-bars";

type DesktopAssociationTableProps = {
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
  onRefreshStatus: () => void;
  onToggleStatus: (association: ModelWithProvider, nextStatus: boolean) => void;
  onEdit: (association: ModelWithProvider) => void;
  onOpenDelete: (id: number) => void;
  onDeleteDialogChange: (open: boolean) => void;
  onDeleteConfirm: () => void;
  onTest: (id: number) => void;
};

export function DesktopAssociationTable({
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
  onRefreshStatus,
  onToggleStatus,
  onEdit,
  onOpenDelete,
  onDeleteDialogChange,
  onDeleteConfirm,
  onTest,
}: DesktopAssociationTableProps) {
  return (
    <div className="hidden lg:block w-full overflow-x-auto">
      <Table className="min-w-[950px]">
        <TableHeader className="z-20 sticky top-0 bg-secondary/80 text-secondary-foreground">
          <TableRow className="group">
            <TableHead className="w-[50px]">
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
            </TableHead>
            <TableHead>ID</TableHead>
            <TableHead>提供商模型</TableHead>
            <TableHead>类型</TableHead>
            <TableHead>提供商</TableHead>
            <TableHead>能力</TableHead>
            <TableHead>权重</TableHead>
            <TableHead>优先级</TableHead>
            <TableHead>启用</TableHead>
            <TableHead>
              <div className="flex items-center gap-1">
                状态
                <Button
                  onClick={onRefreshStatus}
                  variant="ghost"
                  size="icon"
                  aria-label="刷新状态"
                  title="刷新状态"
                  className="rounded-full"
                >
                  <RefreshCw className="size-4" />
                </Button>
              </div>
            </TableHead>
            <TableHead>健康检测</TableHead>
            <TableHead>测试结果</TableHead>
            {/* 操作列钉右：横向滚动时始终可见（窄屏下表格宽于容器且滚动条隐蔽，不钉右则看不到） */}
            <TableHead className="sticky right-0 bg-secondary shadow-[rgba(0,0,0,0.08)_-4px_0_8px_-2px]">
              操作
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {associations.map((association) => {
            const provider = providers.find((item) => item.ID === association.ProviderID);
            const isAssociationEnabled = association.Status ?? true;
            const statusBars = providerStatus[association.ID];
            const healthBars = healthStatus[association.ID];
            const currentResult = associationTestResults[association.ID];

            return (
              <TableRow key={association.ID} className="group">
                <TableCell>
                  <Checkbox
                    checked={selectedAssociationIds.includes(association.ID)}
                    onCheckedChange={(checked) => onSelectOne(association.ID, !!checked)}
                    aria-label={`选择 ${association.ProviderModel}`}
                  />
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{association.ID}</TableCell>
                <TableCell className="max-w-[200px] truncate" title={association.ProviderModel}>
                  {association.ProviderModel}
                </TableCell>
                <TableCell>{provider?.Type ?? "未知"}</TableCell>
                <TableCell>{provider?.Name ?? "未知"}</TableCell>
                <TableCell>
                  <CapabilityBadges
                    toolCall={association.ToolCall}
                    structuredOutput={association.StructuredOutput}
                    image={association.Image}
                    withHeader={association.WithHeader}
                  />
                </TableCell>
                <TableCell>{association.Weight}</TableCell>
                <TableCell>{association.Priority ?? 100}</TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={isAssociationEnabled}
                      disabled={!!statusUpdating[association.ID]}
                      onCheckedChange={(value) => onToggleStatus(association, value)}
                      aria-label="切换启用状态"
                    />
                    <span className="text-xs text-muted-foreground">
                      {isAssociationEnabled ? "已启用" : "已停用"}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center space-x-4 w-20">
                    <StatusBars
                      bars={statusBars}
                      successClassName="bg-success"
                      failClassName="bg-destructive"
                      showTitle
                      successTitle="成功"
                      failTitle="失败"
                    />
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center space-x-4 w-20">
                    <StatusBars
                      bars={healthBars}
                      successClassName="bg-success"
                      failClassName="bg-warning"
                      showTitle
                      successTitle="健康检测成功"
                      failTitle="健康检测失败"
                    />
                  </div>
                </TableCell>
                <TableCell>
                  {currentResult ? (
                    <div className="flex items-center gap-2">
                      {currentResult.loading ? (
                        <>
                          <Spinner className="w-4 h-4" />
                          <span className="text-xs text-muted-foreground">测试中</span>
                        </>
                      ) : currentResult.success === true ? (
                        <span className="text-xs text-success font-medium">✓ 成功</span>
                      ) : currentResult.success === false ? (
                        <span className="text-xs text-destructive font-medium" title={currentResult.error}>
                          ✗ 失败
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    <span className="text-xs text-muted-foreground">-</span>
                  )}
                </TableCell>
                <TableCell className="sticky right-0 bg-background group-hover:bg-muted/50 shadow-[rgba(0,0,0,0.08)_-4px_0_8px_-2px]">
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" size="sm" onClick={() => onEdit(association)}>
                      编辑
                    </Button>
                    <AlertDialog
                      open={deleteId === association.ID}
                      onOpenChange={(open) => onDeleteDialogChange(open)}
                    >
                      <Button variant="destructive" size="sm" onClick={() => onOpenDelete(association.ID)}>
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
                    <Button variant="outline" size="sm" onClick={() => onTest(association.ID)}>
                      测试
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
