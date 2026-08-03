import type { ProviderModelGroup, ProviderModelSelection, ProviderModelWithOwner } from "../../types";
import { buildSelectionKey } from "../../utils/selection";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LoadingState } from "@/components/ui/loading-state";
import { ChevronDown, ChevronRight } from "lucide-react";

type ModelListDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  modelSearchKeyword: string;
  onModelSearchKeywordChange: (value: string) => void;
  loadingProviderModels: boolean;
  providerModels: ProviderModelWithOwner[];
  visibleProviderGroups: ProviderModelGroup[];
  visibleAvailableModels: ProviderModelWithOwner[];
  visibleExistingCount: number;
  selectedProviderModels: ProviderModelSelection[];
  selectedKeys: Set<string>;
  existingAssociationKeys: Set<string>;
  collapsedProviders: Record<number, boolean>;
  onToggleProviderCollapse: (providerId: number) => void;
  onSelectAllVisibleAvailable: () => void;
  onClearSelection: () => void;
  onToggleModelSelection: (
    model: ProviderModelWithOwner,
    checked: boolean,
    selectionKey: string
  ) => void;
};

export function ModelListDialog({
  open,
  onOpenChange,
  modelSearchKeyword,
  onModelSearchKeywordChange,
  loadingProviderModels,
  providerModels,
  visibleProviderGroups,
  visibleAvailableModels,
  visibleExistingCount,
  selectedProviderModels,
  selectedKeys,
  existingAssociationKeys,
  collapsedProviders,
  onToggleProviderCollapse,
  onSelectAllVisibleAvailable,
  onClearSelection,
  onToggleModelSelection,
}: ModelListDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>选择模型</DialogTitle>
          <DialogDescription>从提供商的全部模型缓存中选择要关联的模型</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 flex-1 min-h-0 flex flex-col">
          <div className="flex-shrink-0">
            <Input
              placeholder="搜索模型..."
              value={modelSearchKeyword}
              onChange={(event) => onModelSearchKeywordChange(event.target.value)}
              className="w-full"
            />
          </div>

          <div className="flex items-center justify-between flex-shrink-0">
            <span className="text-sm text-muted-foreground">
              {loadingProviderModels
                ? "加载中..."
                : `共 ${visibleAvailableModels.length} 个可选模型${
                    visibleExistingCount > 0 ? `（${visibleExistingCount} 个已关联）` : ""
                  }`}
            </span>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onSelectAllVisibleAvailable}
                disabled={loadingProviderModels || visibleAvailableModels.length === 0}
              >
                全选可选
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onClearSelection}
                disabled={selectedProviderModels.length === 0}
              >
                清空
              </Button>
            </div>
          </div>

          <div className="flex-1 min-h-0 overflow-y-auto border rounded-md p-2 space-y-2">
            {loadingProviderModels ? (
              <LoadingState text="加载全部模型..." className="text-sm" spinnerClassName="h-4 w-4" />
            ) : providerModels.length === 0 ? (
              <div className="text-center py-4 text-sm text-muted-foreground">暂无全部模型缓存，请先在提供商管理页同步</div>
            ) : visibleProviderGroups.length === 0 ? (
              <div className="text-center py-4 text-sm text-muted-foreground">没有匹配的模型</div>
            ) : (
              visibleProviderGroups.map((group) => {
                const isCollapsed = collapsedProviders[group.provider.ID] ?? false;
                return (
                  <div key={group.provider.ID} className="rounded-md border">
                    <button
                      type="button"
                      className="flex w-full items-center justify-between px-2 py-2 text-left hover:bg-muted/70 transition"
                      onClick={() => onToggleProviderCollapse(group.provider.ID)}
                    >
                      <div className="flex items-center gap-2">
                        {isCollapsed ? (
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        ) : (
                          <ChevronDown className="h-4 w-4 text-muted-foreground" />
                        )}
                        <span className="font-medium">{group.provider.Name}</span>
                      </div>
                      <span className="text-xs text-muted-foreground">{group.models.length} 个模型</span>
                    </button>
                    {!isCollapsed && (
                      <div className="divide-y">
                        {group.models.map((model) => {
                          const selectionKey = buildSelectionKey(model.providerId, model.id);
                          const isExisting = existingAssociationKeys.has(selectionKey);
                          const checked = selectedKeys.has(selectionKey);
                          return (
                            <div
                              key={selectionKey}
                              className={`flex items-center gap-2 px-3 py-2 ${isExisting ? "opacity-60" : ""}`}
                            >
                              <Checkbox
                                id={`dialog-model-${selectionKey}`}
                                checked={checked}
                                disabled={isExisting}
                                onCheckedChange={(checkedValue) => {
                                  onToggleModelSelection(model, checkedValue === true, selectionKey);
                                }}
                              />
                              <div className="min-w-0 flex-1">
                                <Label
                                  htmlFor={`dialog-model-${selectionKey}`}
                                  className={`text-sm cursor-pointer truncate ${isExisting ? "cursor-not-allowed" : ""}`}
                                >
                                  {model.id}
                                  {isExisting && (
                                    <span className="ml-2 text-xs text-muted-foreground">（已关联）</span>
                                  )}
                                </Label>
                                <p className="text-xs text-muted-foreground">提供商：{model.providerName}</p>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>

          <div className="text-xs text-muted-foreground flex-shrink-0">
            已选择 {selectedProviderModels.length} 个模型
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            确定
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

