import { PageToolbar, ToolbarFilter } from "@/components/page-toolbar";
import { Button } from "@/components/ui/button";
import { SelectItem } from "@/components/ui/select";
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

  // 三个「高级」筛选项在窄屏隐藏、改由下方折叠区渲染，故按 className 参数化复用同一份定义
  const modelFilter = (className?: string) => (
    <ToolbarFilter
      label="模型名称"
      value={filters.model}
      onValueChange={(value) => onFilterChange("model", value)}
      placeholder="选择模型"
      className={className}
    >
      <SelectItem value="all">全部</SelectItem>
      {models.map((model) => (
        <SelectItem key={model.ID} value={model.Name}>
          {model.Name}
        </SelectItem>
      ))}
    </ToolbarFilter>
  );

  const providerFilter = (className?: string) => (
    <ToolbarFilter
      label="提供商"
      value={filters.providerName}
      onValueChange={(value) => onFilterChange("providerName", value)}
      placeholder="选择提供商"
      className={className}
    >
      <SelectItem value="all">全部</SelectItem>
      {providers.map((provider) => (
        <SelectItem key={provider.ID} value={provider.Name}>
          {provider.Name}
        </SelectItem>
      ))}
    </ToolbarFilter>
  );

  const statusFilter = (
    <ToolbarFilter
      label="状态"
      value={filters.status}
      onValueChange={(value) => onFilterChange("status", value)}
      placeholder="状态"
    >
      <SelectItem value="all">全部</SelectItem>
      <SelectItem value="success">成功</SelectItem>
      <SelectItem value="error">错误</SelectItem>
    </ToolbarFilter>
  );

  const styleFilter = (
    <ToolbarFilter
      label="类型"
      value={filters.style}
      onValueChange={(value) => onFilterChange("style", value)}
      placeholder="类型"
    >
      <SelectItem value="all">全部</SelectItem>
      {availableStyles.map((style) => (
        <SelectItem key={style} value={style}>
          {style}
        </SelectItem>
      ))}
    </ToolbarFilter>
  );

  const userAgentFilter = (className?: string) => (
    <ToolbarFilter
      label="用户代理"
      value={filters.userAgent}
      onValueChange={(value) => onFilterChange("userAgent", value)}
      placeholder="User Agent"
      className={className}
    >
      <SelectItem value="all">全部</SelectItem>
      {userAgents.map((userAgent) => (
        <SelectItem key={userAgent} value={userAgent}>
          <span className="truncate max-w-[140px] block">
            {userAgent.length > 20 ? `${userAgent.substring(0, 20)}...` : userAgent}
          </span>
        </SelectItem>
      ))}
    </ToolbarFilter>
  );

  return (
    <PageToolbar>
      {modelFilter("hidden sm:flex")}
      {providerFilter("hidden sm:flex")}
      {statusFilter}
      {styleFilter}
      {userAgentFilter("hidden sm:flex")}

      {/* 移动端「更多筛选」折叠区：占满整行，与上方 statusFilter/styleFilter 分行 */}
      <div className="sm:hidden flex w-full flex-col gap-2">
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
          <div className="flex flex-wrap items-end gap-2 pt-2">
            {modelFilter()}
            {providerFilter()}
            {userAgentFilter("basis-full")}
          </div>
        </div>
      </div>
    </PageToolbar>
  );
}
