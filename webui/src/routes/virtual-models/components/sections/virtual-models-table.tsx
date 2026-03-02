import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { VirtualModel } from "@/lib/api";

interface VirtualModelsTableProps {
  virtualModels: VirtualModel[];
  onManageMappings: (model: VirtualModel) => void;
  onManageBlacklist: (model: VirtualModel) => void;
  onEdit: (model: VirtualModel) => void;
  onDelete: (model: VirtualModel) => void;
  getStrategyLabel: (strategy: string) => string;
}

function VirtualModelStatusPill({ enabled }: { enabled: boolean }) {
  return (
    <span
      className={`rounded px-2 py-1 text-xs ${
        enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"
      }`}
    >
      {enabled ? "启用" : "禁用"}
    </span>
  );
}

export function VirtualModelsTable({
  virtualModels,
  onManageMappings,
  onManageBlacklist,
  onEdit,
  onDelete,
  getStrategyLabel,
}: VirtualModelsTableProps) {
  if (virtualModels.length === 0) {
    return (
      <div className="rounded-lg border bg-background">
        <div className="flex items-center justify-center py-10 text-muted-foreground">暂无虚拟模型</div>
      </div>
    );
  }

  return (
    <div className="rounded-lg border bg-background">
      {/* Desktop / Tablet */}
      <div className="hidden sm:block w-full overflow-x-auto">
        <Table className="min-w-[980px]">
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>描述</TableHead>
              <TableHead>路由策略</TableHead>
              <TableHead>重试/超时</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="w-[360px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {virtualModels.map((model) => (
              <TableRow key={model.ID}>
                <TableCell className="font-medium">{model.Name}</TableCell>
                <TableCell className="max-w-xs truncate">{model.Description || "-"}</TableCell>
                <TableCell>{getStrategyLabel(model.Strategy)}</TableCell>
                <TableCell>
                  {model.MaxRetry} / {model.TimeOut}s
                </TableCell>
                <TableCell>
                  <VirtualModelStatusPill enabled={model.Enabled} />
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" size="sm" onClick={() => onManageMappings(model)}>
                      管理映射
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => onManageBlacklist(model)}>
                      拉黑管理
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => onEdit(model)}>
                      编辑
                    </Button>
                    <Button variant="destructive" size="sm" onClick={() => onDelete(model)}>
                      删除
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Mobile */}
      <div className="sm:hidden divide-y divide-border px-3 py-2">
        {virtualModels.map((model) => (
          <div key={model.ID} className="py-3 space-y-2">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="font-semibold text-sm truncate">{model.Name}</div>
                <div className="mt-0.5 text-[11px] text-muted-foreground">
                  策略: <span className="text-foreground">{getStrategyLabel(model.Strategy)}</span>
                </div>
              </div>
              <div className="shrink-0">
                <VirtualModelStatusPill enabled={model.Enabled} />
              </div>
            </div>

            {model.Description && (
              <div className="text-xs text-muted-foreground break-words">{model.Description}</div>
            )}

            <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
              <span>
                重试: <span className="font-medium text-foreground">{model.MaxRetry}</span>
              </span>
              <span>
                超时: <span className="font-medium text-foreground">{model.TimeOut}s</span>
              </span>
            </div>

            <div className="flex flex-wrap justify-end gap-1.5 pt-1">
              <Button
                variant="outline"
                size="sm"
                className="h-7 px-2.5 text-xs"
                onClick={() => onManageMappings(model)}
              >
                管理映射
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="h-7 px-2.5 text-xs"
                onClick={() => onManageBlacklist(model)}
              >
                拉黑管理
              </Button>
              <Button variant="outline" size="sm" className="h-7 px-2.5 text-xs" onClick={() => onEdit(model)}>
                编辑
              </Button>
              <Button variant="destructive" size="sm" className="h-7 px-2.5 text-xs" onClick={() => onDelete(model)}>
                删除
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
