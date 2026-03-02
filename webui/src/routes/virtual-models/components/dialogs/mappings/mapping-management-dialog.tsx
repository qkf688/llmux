import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { VirtualModelMapping } from "@/lib/api";

interface MappingManagementDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  virtualModelName?: string;
  mappings: VirtualModelMapping[];
  getRealModelName: (modelId: number) => string;
  onAddMapping: () => void;
  onOpenBatchDialog: () => void;
  onEditMapping: (mapping: VirtualModelMapping) => void;
  onDeleteMapping: (mappingId: number) => void;
}

export function MappingManagementDialog({
  open,
  onOpenChange,
  virtualModelName,
  mappings,
  getRealModelName,
  onAddMapping,
  onOpenBatchDialog,
  onEditMapping,
  onDeleteMapping,
}: MappingManagementDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[82vh] max-h-[92vh] w-[88vw] max-w-3xl flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>管理映射 - {virtualModelName}</DialogTitle>
          <DialogDescription>配置虚拟模型关联的真实模型</DialogDescription>
        </DialogHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-2">
          <div className="flex gap-2">
            <Button size="sm" className="h-8 px-3" onClick={onAddMapping}>
              添加映射
            </Button>
            <Button size="sm" variant="outline" className="h-8 px-3" onClick={onOpenBatchDialog}>
              批量添加
            </Button>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="py-2 text-xs">真实模型</TableHead>
                  <TableHead className="py-2 text-xs">优先级</TableHead>
                  <TableHead className="py-2 text-xs">权重</TableHead>
                  <TableHead className="py-2 text-xs">状态</TableHead>
                  <TableHead className="py-2 text-xs">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {mappings.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-3 text-center text-sm text-muted-foreground">
                      暂无映射
                    </TableCell>
                  </TableRow>
                ) : (
                  mappings.map((mapping) => (
                    <TableRow key={mapping.ID}>
                      <TableCell className="py-2 text-sm">{getRealModelName(mapping.RealModelID)}</TableCell>
                      <TableCell className="py-2 text-sm">{mapping.Priority}</TableCell>
                      <TableCell className="py-2 text-sm">{mapping.Weight}</TableCell>
                      <TableCell className="py-2">
                        <span
                          className={`rounded px-2 py-0.5 text-xs ${
                            mapping.Enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"
                          }`}
                        >
                          {mapping.Enabled ? "启用" : "禁用"}
                        </span>
                      </TableCell>
                      <TableCell className="py-2">
                        <div className="flex gap-1.5">
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 px-2"
                            onClick={() => onEditMapping(mapping)}
                          >
                            编辑
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            className="h-7 px-2"
                            onClick={() => onDeleteMapping(mapping.ID)}
                          >
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
        </div>
      </DialogContent>
    </Dialog>
  );
}
