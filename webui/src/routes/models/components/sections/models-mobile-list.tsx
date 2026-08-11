import { AnimatePresence } from "motion/react";
import { AnimatedListItem } from "@/components/ui/animated-list-item";
import { StaggerList } from "@/components/ui/stagger-list";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import type { Model } from "@/lib/api";

interface ModelsMobileListProps {
  models: Model[];
  selectedIds: number[];
  isAllSelected: boolean;
  isPartialSelected: boolean;
  togglingIOLog: Record<number, boolean>;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onToggleIOLog: (model: Model) => void;
  onAssociate: (model: Model) => void;
  onEdit: (model: Model) => void;
  onDelete: (model: Model) => void;
}

export function ModelsMobileList({
  models,
  selectedIds,
  isAllSelected,
  isPartialSelected,
  togglingIOLog,
  onSelectAll,
  onSelectOne,
  onToggleIOLog,
  onAssociate,
  onEdit,
  onDelete,
}: ModelsMobileListProps) {
  return (
    <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2">
      <div className="py-2 flex items-center gap-2 border-b">
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
        <span className="text-sm text-muted-foreground">
          {selectedIds.length > 0 ? `已选择 ${selectedIds.length} 项` : "全选"}
        </span>
      </div>

      {/* 外层 StaggerList 编排列表项首屏错峰入场（子项传 staggered 才会参与编排）；
          AnimatePresence 保留运行时增删动画。不传 resetKey：单条增删不重播整批 */}
      <StaggerList>
        <AnimatePresence>
          {models.map((model) => (
            <AnimatedListItem
              key={model.ID}
              staggered
              className="py-2 space-y-2 border-b last:border-b-0"
            >
              <div className="flex items-center gap-2 min-w-0">
                <Checkbox
                  checked={selectedIds.includes(model.ID)}
                  onCheckedChange={(checked) => onSelectOne(model.ID, checked === true)}
                  aria-label={`选择 ${model.Name}`}
                  className="shrink-0"
                />
                <div className="min-w-0 flex-1">
                  <h3 className="font-semibold text-sm truncate">{model.Name}</h3>
                  <p className="text-[10px] text-muted-foreground">ID: {model.ID}</p>
                </div>
              </div>

              <div className="flex items-center gap-3 text-xs flex-wrap">
                <span className="text-muted-foreground">
                  重试: <span className="font-medium text-foreground">{model.MaxRetry}</span>
                </span>
                <span className="text-muted-foreground">
                  超时: <span className="font-medium text-foreground">{model.TimeOut}s</span>
                </span>
                <div className="flex items-center gap-1.5">
                  <span className="text-muted-foreground">IO:</span>
                  <Switch
                    checked={model.IOLog}
                    onCheckedChange={() => onToggleIOLog(model)}
                    disabled={togglingIOLog[model.ID]}
                    className="scale-75"
                  />
                </div>
              </div>

              {model.Remark && (
                <div className="text-xs bg-muted/30 rounded px-2 py-1">
                  <span className="text-muted-foreground">备注: </span>
                  <span className="break-words">{model.Remark}</span>
                </div>
              )}

              <div className="flex justify-end gap-1.5 pt-1">
                <Button variant="secondary" size="sm" className="h-7 px-2.5 text-xs" onClick={() => onAssociate(model)}>
                  关联
                </Button>
                <Button variant="outline" size="sm" className="h-7 px-2.5 text-xs" onClick={() => onEdit(model)}>
                  编辑
                </Button>
                <Button variant="destructive" size="sm" className="h-7 px-2.5 text-xs" onClick={() => onDelete(model)}>
                  删除
                </Button>
              </div>
            </AnimatedListItem>
          ))}
        </AnimatePresence>
      </StaggerList>
    </div>
  );
}
