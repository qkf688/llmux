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
import type { Provider } from "@/lib/api";

interface ProviderSelectorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providers: Provider[];
  selectedProviderIds: number[];
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
  onToggleSelection: (providerId: number) => void;
  onConfirm: () => void;
}

export function ProviderSelectorDialog({
  open,
  onOpenChange,
  providers,
  selectedProviderIds,
  searchQuery,
  onSearchQueryChange,
  onToggleSelection,
  onConfirm,
}: ProviderSelectorDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[78vh] max-h-[90vh] w-[90vw] max-w-2xl flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>选择提供商</DialogTitle>
          <DialogDescription>选择要拉黑的提供商</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-2">
          <div>
            <Input
              className="h-9"
              placeholder="搜索提供商名称..."
              value={searchQuery}
              onChange={(event) => onSearchQueryChange(event.target.value)}
            />
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto rounded-md border">
            <div className="divide-y">
              {providers.map((provider) => {
                const isSelected = selectedProviderIds.includes(provider.ID);
                const isBlacklisted = provider.blacklisted;
                return (
                  <div
                    key={provider.ID}
                    className={`flex items-center gap-2 p-2 hover:bg-muted/50 ${
                      isBlacklisted ? "bg-muted" : ""
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => onToggleSelection(provider.ID)}
                      disabled={isBlacklisted}
                      className="h-3.5 w-3.5"
                    />
                    <div className="flex-1">
                      <div
                        className={`text-sm font-medium ${
                          isBlacklisted ? "text-muted-foreground line-through" : ""
                        }`}
                      >
                        {provider.Name}
                      </div>
                      <div className="text-xs text-muted-foreground">{provider.Type}</div>
                    </div>
                    {isBlacklisted && (
                      <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                        已拉黑
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={onConfirm}>添加 ({selectedProviderIds.length})</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
