import { PageToolbar, ToolbarFilter } from "@/components/page-toolbar";
import { SelectItem } from "@/components/ui/select";
import type { Model, Provider } from "@/lib/api";
import type { HealthCheckLogsFilters } from "../../types";

type HealthCheckFiltersProps = {
  filters: HealthCheckLogsFilters;
  models: Model[];
  providers: Provider[];
  onFilterChange: (key: keyof HealthCheckLogsFilters, value: string) => void;
};

export function HealthCheckFilters({
  filters,
  models,
  providers,
  onFilterChange,
}: HealthCheckFiltersProps) {
  return (
    <PageToolbar>
      <ToolbarFilter
        label="模型名称"
        value={filters.modelName}
        onValueChange={(value) => onFilterChange("modelName", value)}
        placeholder="选择模型"
      >
        <SelectItem value="all">全部</SelectItem>
        {models.map((model) => (
          <SelectItem key={model.ID} value={model.Name}>
            {model.Name}
          </SelectItem>
        ))}
      </ToolbarFilter>

      <ToolbarFilter
        label="提供商"
        value={filters.providerName}
        onValueChange={(value) => onFilterChange("providerName", value)}
        placeholder="选择提供商"
      >
        <SelectItem value="all">全部</SelectItem>
        {providers.map((provider) => (
          <SelectItem key={provider.ID} value={provider.Name}>
            {provider.Name}
          </SelectItem>
        ))}
      </ToolbarFilter>

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
    </PageToolbar>
  );
}
