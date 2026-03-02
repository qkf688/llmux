import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { Model } from "@/lib/api";

interface ModelsDesktopTableProps {
  models: Model[];
  selectedIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  togglingIOLog: Record<number, boolean>;
  togglingAutoAssociate: Record<number, boolean>;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onToggleIOLog: (model: Model) => void;
  onToggleAutoAssociate: (model: Model, checked: boolean) => void;
  onAssociate: (model: Model) => void;
  onEdit: (model: Model) => void;
  onDelete: (model: Model) => void;
}

export function ModelsDesktopTable({
  models,
  selectedIds,
  isAllSelected,
  isPartialSelected,
  togglingIOLog,
  togglingAutoAssociate,
  onSelectAll,
  onSelectOne,
  onToggleIOLog,
  onToggleAutoAssociate,
  onAssociate,
  onEdit,
  onDelete,
}: ModelsDesktopTableProps) {
  return (
    <div className="hidden sm:block w-full overflow-x-auto">
      <Table className="min-w-[900px]">
        <TableHeader className="z-10 sticky top-0 bg-secondary/80 text-secondary-foreground">
          <TableRow>
            <TableHead className="w-[50px]">
              <Checkbox
                checked={isAllSelected}
                ref={(element) => {
                  if (element) {
                    (element as unknown as HTMLInputElement).indeterminate = isPartialSelected;
                  }
                }}
                onCheckedChange={(checked) => onSelectAll(checked === true)}
                aria-label="全选"
              />
            </TableHead>
            <TableHead>ID</TableHead>
            <TableHead>名称</TableHead>
            <TableHead>备注</TableHead>
            <TableHead>重试次数限制</TableHead>
            <TableHead>超时时间(秒)</TableHead>
            <TableHead>IO 记录</TableHead>
            <TableHead>自动关联</TableHead>
            <TableHead className="w-[220px]">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {models.map((model) => (
            <TableRow key={model.ID}>
              <TableCell>
                <Checkbox
                  checked={selectedIds.includes(model.ID)}
                  onCheckedChange={(checked) => onSelectOne(model.ID, checked === true)}
                  aria-label={`选择 ${model.Name}`}
                />
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">{model.ID}</TableCell>
              <TableCell className="font-medium">{model.Name}</TableCell>
              <TableCell className="max-w-[240px] truncate text-sm" title={model.Remark}>
                {model.Remark || "-"}
              </TableCell>
              <TableCell>{model.MaxRetry}</TableCell>
              <TableCell>{model.TimeOut}</TableCell>
              <TableCell>
                <Switch
                  checked={model.IOLog}
                  onCheckedChange={() => onToggleIOLog(model)}
                  disabled={togglingIOLog[model.ID]}
                />
              </TableCell>
              <TableCell>
                <Switch
                  checked={model.auto_associate !== false}
                  onCheckedChange={(checked) => onToggleAutoAssociate(model, checked)}
                  disabled={togglingAutoAssociate[model.ID]}
                />
              </TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-2">
                  <Button variant="secondary" size="sm" onClick={() => onAssociate(model)}>
                    关联
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
  );
}
