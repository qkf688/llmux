import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import type { Model } from "@/lib/api";

interface MappingBatchDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: Model[];
  mappedModelIds: Set<number>;
  selectedModelIds: number[];
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
  onSelectAll: () => void;
  onInvertSelection: () => void;
  onClearSelection: () => void;
  onToggleModelSelection: (modelId: number) => void;
  batchPriority: number;
  onBatchPriorityChange: (value: number) => void;
  batchWeight: number;
  onBatchWeightChange: (value: number) => void;
  batchEnabled: boolean;
  onBatchEnabledChange: (enabled: boolean) => void;
  onSubmit: () => void;
}

const parseInteger = (value: string) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
};

export function MappingBatchDialog({
  open,
  onOpenChange,
  models,
  mappedModelIds,
  selectedModelIds,
  searchQuery,
  onSearchQueryChange,
  onSelectAll,
  onInvertSelection,
  onClearSelection,
  onToggleModelSelection,
  batchPriority,
  onBatchPriorityChange,
  batchWeight,
  onBatchWeightChange,
  batchEnabled,
  onBatchEnabledChange,
  onSubmit,
}: MappingBatchDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[80vh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>批量添加映射</DialogTitle>
          <DialogDescription>选择多个真实模型并设置统一参数</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Input
              placeholder="搜索模型名称..."
              value={searchQuery}
              onChange={(event) => onSearchQueryChange(event.target.value)}
            />
          </div>

          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={onSelectAll}>
              全选
            </Button>
            <Button variant="outline" size="sm" onClick={onInvertSelection}>
              反选
            </Button>
            <Button variant="outline" size="sm" onClick={onClearSelection}>
              清空
            </Button>
            <span className="ml-auto self-center text-sm text-muted-foreground">
              已选择 {selectedModelIds.length} 个模型
            </span>
          </div>

          <div className="max-h-60 overflow-y-auto rounded-lg border">
            <div className="divide-y">
              {models.map((model) => {
                const isMapped = mappedModelIds.has(model.ID);
                const isSelected = selectedModelIds.includes(model.ID);
                return (
                  <div
                    key={model.ID}
                    className={`flex items-center gap-3 p-3 hover:bg-gray-50 ${
                      isMapped ? "bg-gray-100" : ""
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => onToggleModelSelection(model.ID)}
                      disabled={isMapped}
                      className="h-4 w-4"
                    />
                    <div className="flex-1">
                      <div className={`font-medium ${isMapped ? "text-gray-500 line-through" : ""}`}>
                        {model.Name}
                      </div>
                      {model.Remark && <div className="text-sm text-muted-foreground">{model.Remark}</div>}
                    </div>
                    {isMapped && (
                      <span className="rounded bg-gray-200 px-2 py-1 text-xs text-gray-500">已映射</span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="space-y-3 border-t pt-4">
            <h4 className="font-medium">统一参数</h4>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium">优先级</label>
                <Input
                  type="number"
                  value={batchPriority}
                  onChange={(event) => onBatchPriorityChange(parseInteger(event.target.value))}
                />
              </div>
              <div>
                <label className="text-sm font-medium">权重</label>
                <Input
                  type="number"
                  value={batchWeight}
                  onChange={(event) => onBatchWeightChange(parseInteger(event.target.value))}
                />
              </div>
            </div>

            <div className="flex items-center gap-2">
              <label className="text-sm font-medium">启用</label>
              <Switch checked={batchEnabled} onCheckedChange={onBatchEnabledChange} />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={onSubmit}>添加 ({selectedModelIds.length})</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
