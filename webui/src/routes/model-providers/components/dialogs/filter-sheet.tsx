import type { Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { X } from "lucide-react";

type FilterSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedProviderType: string;
  onSelectedProviderTypeChange: (value: string) => void;
  selectedProviderFilter: string;
  onSelectedProviderFilterChange: (value: string) => void;
  selectedStatusFilter: string;
  onSelectedStatusFilterChange: (value: string) => void;
  providers: Provider[];
  providerTypes: string[];
  onReset: () => void;
};

export function FilterSheet({
  open,
  onOpenChange,
  selectedProviderType,
  onSelectedProviderTypeChange,
  selectedProviderFilter,
  onSelectedProviderFilterChange,
  selectedStatusFilter,
  onSelectedStatusFilterChange,
  providers,
  providerTypes,
  onReset,
}: FilterSheetProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="sheet" showCloseButton={false}>
        <div className="flex min-h-0 flex-1 flex-col">
          <DialogHeader className="px-4 py-3 border-b">
            <div className="flex items-center justify-between">
              <DialogTitle className="text-base">筛选</DialogTitle>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => onOpenChange(false)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
            <DialogDescription className="text-xs">
              按提供商类型、具体提供商或启用状态筛选关联
            </DialogDescription>
          </DialogHeader>

          <DialogBody className="p-4 space-y-4">
            <div className="flex flex-col gap-1 text-xs">
              <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商类型</Label>
              <Select value={selectedProviderType} onValueChange={onSelectedProviderTypeChange}>
                <SelectTrigger className="h-9 w-full text-xs px-2">
                  <SelectValue placeholder="按类型筛选" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部类型</SelectItem>
                  {providerTypes.map((type) => (
                    <SelectItem key={type} value={type}>
                      {type}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-1 text-xs">
              <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">具体提供商</Label>
              <Select value={selectedProviderFilter} onValueChange={onSelectedProviderFilterChange}>
                <SelectTrigger className="h-9 w-full text-xs px-2">
                  <SelectValue placeholder="按提供商筛选" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部提供商</SelectItem>
                  {providers.map((provider) => (
                    <SelectItem key={provider.ID} value={provider.ID.toString()}>
                      {provider.Name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-1 text-xs">
              <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">启用状态</Label>
              <Select value={selectedStatusFilter} onValueChange={onSelectedStatusFilterChange}>
                <SelectTrigger className="h-9 w-full text-xs px-2">
                  <SelectValue placeholder="按状态筛选" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部状态</SelectItem>
                  <SelectItem value="enabled">已启用</SelectItem>
                  <SelectItem value="disabled">未启用</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </DialogBody>

          <div className="flex shrink-0 gap-2 p-4 border-t">
            <Button
              variant="outline"
              className="flex-1 h-9"
              onClick={onReset}
            >
              重置
            </Button>
            <Button
              className="flex-1 h-9"
              onClick={() => onOpenChange(false)}
            >
              确定
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
