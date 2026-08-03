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
import { LoadingState } from "@/components/ui/loading-state";
import type { Provider } from "@/lib/api";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { ProviderModelGroup, ProviderModelWithOwner } from "../../types";

interface ModelPickerDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedProviderId: string;
  providers: Provider[];
  loadingProviderModels: boolean;
  providerModels: ProviderModelWithOwner[];
  filteredProviderGroups: ProviderModelGroup[];
  collapsedProviders: Record<number, boolean>;
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
  onToggleProviderCollapse: (providerId: number) => void;
  onSelectModel: (modelId: string) => void;
}

export function ModelPickerDialog({
  open,
  onOpenChange,
  selectedProviderId,
  providers,
  loadingProviderModels,
  providerModels,
  filteredProviderGroups,
  collapsedProviders,
  searchQuery,
  onSearchQueryChange,
  onToggleProviderCollapse,
  onSelectModel,
}: ModelPickerDialogProps) {
  const filteredProviderModels = filteredProviderGroups.flatMap((group) => group.models);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>选择模型</DialogTitle>
          <DialogDescription>
            {selectedProviderId === "all"
              ? "从全部提供商的模型缓存中选择"
              : `从供应商 ${
                  providers.find((provider) => provider.ID.toString() === selectedProviderId)?.Name ?? "未找到"
                } 的模型缓存中选择`}
          </DialogDescription>
        </DialogHeader>

        <div className="py-2">
          <Input
            placeholder="搜索模型名称..."
            value={searchQuery}
            onChange={(event) => onSearchQueryChange(event.target.value)}
            className="w-full"
          />
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto border rounded-md space-y-2 p-2">
          {loadingProviderModels ? (
            <LoadingState text="加载模型列表..." className="h-32" spinnerClassName="h-6 w-6" />
          ) : providerModels.length === 0 ? (
            <div className="flex items-center justify-center h-32 text-muted-foreground">
              暂无全部模型缓存，请先在提供商管理页同步
            </div>
          ) : filteredProviderGroups.length === 0 ? (
            <div className="flex items-center justify-center h-32 text-muted-foreground">没有找到匹配的模型</div>
          ) : (
            filteredProviderGroups.map((group) => {
              const isCollapsed = collapsedProviders[group.provider.ID] ?? false;

              return (
                <div key={group.provider.ID} className="border rounded-md">
                  <button
                    type="button"
                    className="flex w-full items-center justify-between px-3 py-2 text-left hover:bg-muted/70 transition"
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
                      {group.models.map((model) => (
                        <div
                          key={`${group.provider.ID}-${model.id}`}
                          className="p-3 hover:bg-muted cursor-pointer transition-colors"
                          onClick={() => onSelectModel(model.id)}
                        >
                          <div className="font-medium text-sm">{model.id}</div>
                          <div className="text-xs text-muted-foreground mt-1">
                            提供商: {group.provider.Name}
                            {model.owned_by ? ` · 归属: ${model.owned_by}` : ""}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>

        <DialogFooter>
          <div className="flex items-center justify-between w-full">
            <span className="text-sm text-muted-foreground">
              共 {filteredProviderModels.length} 个模型
              {searchQuery && providerModels.length !== filteredProviderModels.length
                ? ` (已筛选，总计 ${providerModels.length} 个)`
                : ""}
            </span>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              取消
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
