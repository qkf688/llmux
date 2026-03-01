import type { BlacklistFilter } from "../../types";
import type { Provider } from "@/lib/api";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";

type BlacklistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providers: Provider[];
  filteredProviders: Provider[];
  blacklistedIds: number[];
  loading: boolean;
  saving: boolean;
  searchTerm: string;
  filter: BlacklistFilter;
  onSearchTermChange: (value: string) => void;
  onFilterChange: (value: BlacklistFilter) => void;
  onToggle: (providerId: number, checked: boolean) => void;
  onSave: () => void;
  onCancel: () => void;
};

export function BlacklistDialog({
  open,
  onOpenChange,
  providers,
  filteredProviders,
  blacklistedIds,
  loading,
  saving,
  searchTerm,
  filter,
  onSearchTermChange,
  onFilterChange,
  onToggle,
  onSave,
  onCancel,
}: BlacklistDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>拉黑管理</DialogTitle>
          <DialogDescription>
            被拉黑的供应商在一键关联和自动关联时会被跳过，不会关联其模型。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 pt-2">
          <div className="flex gap-2">
            <Input
              placeholder="搜索供应商名称或类型..."
              value={searchTerm}
              onChange={(event) => onSearchTermChange(event.target.value)}
              className="flex-1 h-8 text-sm"
            />
            <Select value={filter} onValueChange={(value) => onFilterChange(value as BlacklistFilter)}>
              <SelectTrigger className="w-24 h-8 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                <SelectItem value="blacklisted">已拉黑</SelectItem>
                <SelectItem value="not-blacklisted">未拉黑</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-8 gap-2 text-sm text-muted-foreground">
            <Spinner className="h-4 w-4" />
            加载中...
          </div>
        ) : (
          <div className="space-y-2">
            {filteredProviders.length === 0 ? (
              <div className="text-sm text-muted-foreground py-4 text-center h-80 flex items-center justify-center">
                {searchTerm || filter !== "all" ? "未找到匹配的供应商" : "暂无供应商"}
              </div>
            ) : (
              <div className="max-h-80 overflow-auto rounded-md border divide-y h-80">
                {filteredProviders.map((provider) => {
                  const isBlacklisted = blacklistedIds.includes(provider.ID);
                  return (
                    <div
                      key={provider.ID}
                      className="flex items-center gap-3 px-4 py-3 hover:bg-muted/50 transition-colors"
                    >
                      <Checkbox
                        id={`blacklist-provider-${provider.ID}`}
                        checked={isBlacklisted}
                        onCheckedChange={(checked) => onToggle(provider.ID, checked === true)}
                        disabled={saving}
                      />
                      <label htmlFor={`blacklist-provider-${provider.ID}`} className="flex-1 cursor-pointer select-none">
                        <span className="text-sm font-medium">{provider.Name}</span>
                        <span className="text-xs text-muted-foreground ml-2">({provider.Type})</span>
                      </label>
                      {isBlacklisted && <span className="text-xs text-destructive font-medium">已拉黑</span>}
                    </div>
                  );
                })}
              </div>
            )}
            <div className="text-xs text-muted-foreground">
              {searchTerm || filter !== "all"
                ? `筛选到 ${filteredProviders.length} 个供应商，已选 ${blacklistedIds.length} 个加入黑名单`
                : `已选 ${blacklistedIds.length} 个供应商加入黑名单`}
            </div>
            {providers.length === 0 && (
              <div className="text-xs text-muted-foreground">当前未读取到可管理的供应商。</div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={saving}>
            取消
          </Button>
          <Button onClick={onSave} disabled={loading || saving}>
            {saving ? (
              <>
                <Spinner className="h-4 w-4 mr-2" />
                保存中...
              </>
            ) : (
              "保存"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

