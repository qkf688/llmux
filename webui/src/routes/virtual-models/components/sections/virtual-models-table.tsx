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

export function VirtualModelsTable({
  virtualModels,
  onManageMappings,
  onManageBlacklist,
  onEdit,
  onDelete,
  getStrategyLabel,
}: VirtualModelsTableProps) {
  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>名称</TableHead>
            <TableHead>描述</TableHead>
            <TableHead>路由策略</TableHead>
            <TableHead>重试/超时</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {virtualModels.length === 0 ? (
            <TableRow>
              <TableCell colSpan={6} className="text-center text-muted-foreground">
                暂无虚拟模型
              </TableCell>
            </TableRow>
          ) : (
            virtualModels.map((model) => (
              <TableRow key={model.ID}>
                <TableCell className="font-medium">{model.Name}</TableCell>
                <TableCell className="max-w-xs truncate">{model.Description || "-"}</TableCell>
                <TableCell>{getStrategyLabel(model.Strategy)}</TableCell>
                <TableCell>
                  {model.MaxRetry} / {model.TimeOut}s
                </TableCell>
                <TableCell>
                  <span
                    className={`rounded px-2 py-1 text-xs ${
                      model.Enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"
                    }`}
                  >
                    {model.Enabled ? "启用" : "禁用"}
                  </span>
                </TableCell>
                <TableCell>
                  <div className="flex gap-2">
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
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
