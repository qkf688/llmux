import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Model, Provider } from "@/lib/api";
import type { LogsFilters } from "../../types";

type LogsFiltersSectionProps = {
  filters: LogsFilters;
  models: Model[];
  providers: Provider[];
  userAgents: string[];
  availableStyles: string[];
  onFilterChange: (key: keyof LogsFilters, value: string) => void;
};

export function LogsFiltersSection({
  filters,
  models,
  providers,
  userAgents,
  availableStyles,
  onFilterChange,
}: LogsFiltersSectionProps) {
  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5 lg:gap-4">
        <div className="flex flex-col gap-1 text-xs lg:min-w-0">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">模型名称</Label>
          <Select value={filters.model} onValueChange={(value) => onFilterChange("model", value)}>
            <SelectTrigger className="h-8 text-xs w-full px-2">
              <SelectValue placeholder="选择模型" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              {models.map((model) => (
                <SelectItem key={model.ID} value={model.Name}>
                  {model.Name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1 text-xs lg:min-w-0">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商</Label>
          <Select
            value={filters.providerName}
            onValueChange={(value) => onFilterChange("providerName", value)}
          >
            <SelectTrigger className="h-8 text-xs w-full px-2">
              <SelectValue placeholder="选择提供商" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              {providers.map((provider) => (
                <SelectItem key={provider.ID} value={provider.Name}>
                  {provider.Name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1 text-xs lg:min-w-0">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">状态</Label>
          <Select value={filters.status} onValueChange={(value) => onFilterChange("status", value)}>
            <SelectTrigger className="h-8 text-xs w-full px-2">
              <SelectValue placeholder="状态" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              <SelectItem value="success">成功</SelectItem>
              <SelectItem value="error">错误</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1 text-xs lg:min-w-0">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">类型</Label>
          <Select value={filters.style} onValueChange={(value) => onFilterChange("style", value)}>
            <SelectTrigger className="h-8 text-xs w-full px-2">
              <SelectValue placeholder="类型" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              {availableStyles.map((style) => (
                <SelectItem key={style} value={style}>
                  {style}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1 text-xs col-span-2 sm:col-span-1 lg:col-span-1 lg:min-w-0">
          <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">用户代理</Label>
          <Select value={filters.userAgent} onValueChange={(value) => onFilterChange("userAgent", value)}>
            <SelectTrigger className="h-8 text-xs w-full px-2">
              <SelectValue placeholder="User Agent" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部</SelectItem>
              {userAgents.map((userAgent) => (
                <SelectItem key={userAgent} value={userAgent}>
                  <span className="truncate max-w-[140px] block">
                    {userAgent.length > 20 ? `${userAgent.substring(0, 20)}...` : userAgent}
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  );
}
