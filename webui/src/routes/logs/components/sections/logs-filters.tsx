import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Model, Provider } from "@/lib/api";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronUp, Filter } from "lucide-react";
import { useMemo, useState } from "react";
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
  const advancedActiveFilterCount = useMemo(() => {
    return [filters.model, filters.providerName, filters.userAgent].filter((value) => value !== "all").length;
  }, [filters.model, filters.providerName, filters.userAgent]);

  // 移动端默认收起，桌面端默认展开（与其它筛选面板保持一致）
  const [filterPanelOpen, setFilterPanelOpen] = useState(() => {
    if (typeof window !== "undefined") {
      return window.innerWidth >= 640; // sm 断点
    }
    return true;
  });

  const modelFilter = (
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
  );

  const providerFilter = (
    <div className="flex flex-col gap-1 text-xs lg:min-w-0">
      <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商</Label>
      <Select value={filters.providerName} onValueChange={(value) => onFilterChange("providerName", value)}>
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
  );

  const statusFilter = (
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
  );

  const styleFilter = (
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
  );

  const userAgentFilter = (className?: string) => (
    <div className={cn("flex flex-col gap-1 text-xs lg:min-w-0", className)}>
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
  );

  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      <div className="hidden sm:block">
        <div className="grid grid-cols-3 gap-2 lg:grid-cols-5">
          {modelFilter}
          {providerFilter}
          {statusFilter}
          {styleFilter}
          {userAgentFilter()}
        </div>
      </div>

      <div className="sm:hidden flex flex-col gap-2">
        <div className="grid grid-cols-2 gap-2">
          {statusFilter}
          {styleFilter}
        </div>

        <Button
          variant="outline"
          size="sm"
          onClick={() => setFilterPanelOpen((open) => !open)}
          className="w-full justify-between h-8 text-xs"
        >
          <span className="flex items-center gap-2">
            <Filter className="h-4 w-4" />
            <span>更多筛选</span>
            {advancedActiveFilterCount > 0 && (
              <span className="bg-primary text-primary-foreground text-xs px-1.5 py-0.5 rounded-full">
                {advancedActiveFilterCount}
              </span>
            )}
          </span>
          {filterPanelOpen ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
        </Button>

        <div
          className={cn(
            "overflow-hidden transition-all duration-300 ease-in-out",
            filterPanelOpen ? "max-h-[500px] opacity-100" : "max-h-0 opacity-0"
          )}
        >
          <div className="grid grid-cols-2 gap-2 pt-2">
            {modelFilter}
            {providerFilter}
            {userAgentFilter("col-span-2")}
          </div>
        </div>
      </div>
    </div>
  );
}
