import type { AssociationBatchTestResult } from "../../../types";
import type { HealthCheckStatus, ModelWithProvider, Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
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
  STICKY_ACTIONS_CELL_CLS,
  STICKY_ACTIONS_HEAD_CLS,
  STICKY_HEADER_CLS,
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
  modelCaps?: { tool_call: boolean; structured_output: boolean; image: boolean };
  providers: Provider[];
  selectedAssociationIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  providerStatus: Record<number, boolean[]>;
  healthStatus: Record<number, HealthCheckStatus[]>;
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
  modelCaps,
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
      {/* 固定布局：列宽只由 colgroup 决定，不再随单元格内容长度变化——否则长模型名/供应商名会挤窄
          「能力」「操作」列，触发内部 flex-wrap 换行，同一张表里行高差 2 倍以上。
          min-w 必须 ≥ 各列宽之和 + 模型名列的可读宽度，否则固定布局会按比例压缩列宽，把挤压缩回来。 */}
      <Table className="table-fixed min-w-[1400px]">
        <colgroup>
          <col className="w-[44px]" />
          <col className="w-[48px]" />
          {/* 提供商模型列不设宽：由它吸收宽屏下的剩余空间（取值来自表宽，仍与内容无关） */}
          <col />
          <col className="w-[72px]" />
          <col className="w-[130px]" />
          <col className="w-[186px]" />
          <col className="w-[56px]" />
          <col className="w-[68px]" />
          <col className="w-[110px]" />
          <col className="w-[108px]" />
          <col className="w-[108px]" />
          <col className="w-[84px]" />
          <col className="w-[212px]" />
        </colgroup>
        <TableHeader className={`z-20 ${STICKY_HEADER_CLS}`}>
          <TableRow className="group">
            <TableHead>
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
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex">
                      <Button
                        onClick={onRefreshStatus}
                        variant="ghost"
                        size="icon"
                        aria-label="刷新状态"
                        className="rounded-full"
                      >
                        <RefreshCw className="size-4" />
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>刷新状态</TooltipContent>
                </Tooltip>
              </div>
            </TableHead>
            <TableHead>健康检测</TableHead>
            <TableHead>测试结果</TableHead>
            {/* 操作列钉右：横向滚动时始终可见（窄屏下表格宽于容器且滚动条隐蔽，不钉右则看不到） */}
            <TableHead className={STICKY_ACTIONS_HEAD_CLS}>
              操作
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {associations.map((association) => {
            const provider = providers.find((item) => item.ID === association.ProviderID);
            const isAssociationEnabled = association.Status ?? true;
            // 聊天成败是两态布尔，传进格子前显式映射成三态词汇（组件只认一套词汇）
        const statusBars = providerStatus[association.ID]?.map((ok): HealthCheckStatus => (ok ? "success" : "error"));
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
                {/* 截断必须落在内层块级元素上：max-width 不被 table-cell 的布局算法采信，
                    直接加在 td 上等于没截断，长名字反而撑宽该列去挤邻列 */}
                <TableCell title={association.ProviderModel}>
                  <span className="block truncate">{association.ProviderModel}</span>
                </TableCell>
                <TableCell>{provider?.Type ?? "未知"}</TableCell>
                <TableCell title={provider?.Name}>
                  <span className="block truncate">{provider?.Name ?? "未知"}</span>
                </TableCell>
                <TableCell>
                  {/* 徽章不换行：列宽已固定，禁止 wrap 才能保证任何数据下行高一致。
                      徽章已收敛为「最终支持的能力」（最多三个 2 字短徽章），
                      宽度与改动前持平，不会溢出压住权重/优先级。 */}
                  <CapabilityBadges
                    nowrap
                    toolCall={association.ToolCall}
                    structuredOutput={association.StructuredOutput}
                    image={association.Image}
                    withHeader={association.WithHeader}
                    modelCaps={modelCaps}
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
                      ) : currentResult.cancelled ? (
                        <span className="text-xs text-muted-foreground font-medium">— 已取消</span>
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
                <TableCell className={STICKY_ACTIONS_CELL_CLS}>
                  <div className="flex flex-nowrap gap-2">
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
