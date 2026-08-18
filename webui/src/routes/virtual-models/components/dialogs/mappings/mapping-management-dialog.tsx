import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
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
  totalMappingsCount: number;
  mappings: VirtualModelMapping[];
  getRealModelName: (modelId: number) => string;
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
  selectedMappingIds: Set<number>;
  isAllFilteredSelected: boolean;
  isSomeFilteredSelected: boolean;
  onSelectAllFiltered: (checked: boolean) => void;
  onToggleMappingSelection: (mappingId: number) => void;
  onOpenBatchDialog: () => void;
  onOpenBatchDeleteDialog: () => void;
  onEditMapping: (mapping: VirtualModelMapping) => void;
  onDeleteMapping: (mappingId: number) => void;
}

export function MappingManagementDialog({
  open,
  onOpenChange,
  virtualModelName,
  totalMappingsCount,
  mappings,
  getRealModelName,
  searchQuery,
  onSearchQueryChange,
  selectedMappingIds,
  isAllFilteredSelected,
  isSomeFilteredSelected,
  onSelectAllFiltered,
  onToggleMappingSelection,
  onOpenBatchDialog,
  onOpenBatchDeleteDialog,
  onEditMapping,
  onDeleteMapping,
}: MappingManagementDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="xl" className="gap-3 p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle className="text-base sm:text-lg">管理映射 - {virtualModelName}</DialogTitle>
          <DialogDescription className="text-xs sm:text-sm">配置虚拟模型关联的真实模型</DialogDescription>
        </DialogHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-2">
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <div className="flex-1 min-w-[200px]">
              <Input
                placeholder="搜索真实模型..."
                value={searchQuery}
                onChange={(event) => onSearchQueryChange(event.target.value)}
              />
            </div>

            <span className="self-center text-sm text-muted-foreground">
              已选择 {selectedMappingIds.size} 条
            </span>
            <Button size="sm" className="h-8 px-3" onClick={onOpenBatchDialog}>
              添加映射
            </Button>
            <Button
              variant="destructive"
              size="sm"
              className="h-8 px-3"
              disabled={selectedMappingIds.size === 0}
              onClick={onOpenBatchDeleteDialog}
            >
              批量删除 ({selectedMappingIds.size})
            </Button>
          </div>

          <DialogBody className="rounded-md border">
            {/* Desktop/tablet: table layout */}
            <div className="hidden sm:block">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="py-2 text-xs w-[40px]">
                      <Checkbox
                        checked={isSomeFilteredSelected ? "indeterminate" : isAllFilteredSelected}
                        onCheckedChange={(checked) => onSelectAllFiltered(checked === true)}
                        aria-label="全选筛选结果"
                      />
                    </TableHead>
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
                      <TableCell colSpan={6} className="py-3 text-center text-sm text-muted-foreground">
                        {totalMappingsCount === 0 ? "暂无映射" : "无匹配映射"}
                      </TableCell>
                    </TableRow>
                  ) : (
                    mappings.map((mapping) => (
                      <TableRow key={mapping.ID} className={selectedMappingIds.has(mapping.ID) ? "bg-muted/50" : ""}>
                        <TableCell className="py-2">
                          <Checkbox
                            checked={selectedMappingIds.has(mapping.ID)}
                            onCheckedChange={() => onToggleMappingSelection(mapping.ID)}
                            aria-label={`选择映射 ${mapping.ID}`}
                          />
                        </TableCell>
                        <TableCell className="py-2 text-sm">{getRealModelName(mapping.RealModelID)}</TableCell>
                        <TableCell className="py-2 text-sm">{mapping.Priority}</TableCell>
                        <TableCell className="py-2 text-sm">{mapping.Weight}</TableCell>
                        <TableCell className="py-2">
                          <span
                            className={`rounded px-2 py-0.5 text-xs ${
                              mapping.Enabled
                                ? "bg-primary/10 border border-primary/30 text-primary"
                                : "bg-muted text-muted-foreground"
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

            {/* Mobile: compact card list (avoids horizontal overflow/clipping) */}
            <div className="divide-y sm:hidden">
              {mappings.length === 0 ? (
                <div className="py-3 text-center text-sm text-muted-foreground">
                  {totalMappingsCount === 0 ? "暂无映射" : "无匹配映射"}
                </div>
              ) : (
                mappings.map((mapping) => {
                  const enabled = mapping.Enabled;
                  return (
                    <div
                      key={mapping.ID}
                      className={`p-2.5 ${selectedMappingIds.has(mapping.ID) ? "bg-muted/50" : ""}`}
                    >
                      <div className="flex items-start gap-2">
                        <Checkbox
                          checked={selectedMappingIds.has(mapping.ID)}
                          onCheckedChange={() => onToggleMappingSelection(mapping.ID)}
                          className="mt-0.5 shrink-0"
                          aria-label={`选择映射 ${mapping.ID}`}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-medium">
                            {getRealModelName(mapping.RealModelID)}
                          </div>
                          <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                            <span>优先级 {mapping.Priority}</span>
                            <span>权重 {mapping.Weight}</span>
                          </div>
                        </div>
                        <span
                          className={`shrink-0 rounded px-2 py-0.5 text-xs ${
                            enabled
                              ? "bg-primary/10 border border-primary/30 text-primary"
                              : "bg-muted text-muted-foreground"
                          }`}
                        >
                          {enabled ? "启用" : "禁用"}
                        </span>
                      </div>

                      <div className="mt-2 flex flex-wrap justify-end gap-2">
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
                    </div>
                  );
                })
              )}
            </div>
          </DialogBody>
        </div>
      </DialogContent>
    </Dialog>
  );
}
