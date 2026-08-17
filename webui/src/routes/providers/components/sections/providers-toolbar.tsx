import { PageToolbar, ToolbarFilter, ToolbarSearch } from "@/components/page-toolbar";
import { SelectItem } from "@/components/ui/select";

interface ProvidersToolbarProps {
  nameFilter: string;
  setNameFilter: (value: string) => void;

  typeFilter: string;
  setTypeFilter: (value: string) => void;
  availableTypes: string[];

  flushNameFilter: () => void;
}

/** providers 筛选栏：名称搜索 + 类型筛选，页头职责在 ProvidersHeader */
export function ProvidersToolbar({
  nameFilter,
  setNameFilter,
  typeFilter,
  setTypeFilter,
  availableTypes,
  flushNameFilter,
}: ProvidersToolbarProps) {
  return (
    <PageToolbar>
      <ToolbarSearch
        ariaLabel="搜索提供商名称"
        placeholder="搜索名称"
        value={nameFilter}
        onChange={setNameFilter}
      />
      <ToolbarFilter
        placeholder="类型"
        value={typeFilter}
        onValueChange={(value) => {
          setTypeFilter(value);
          // 切换类型时立刻应用搜索词，避免防抖期内筛选条件不一致
          flushNameFilter();
        }}
        className="flex-none min-w-[160px]"
      >
        <SelectItem value="all">全部</SelectItem>
        {availableTypes.map((type) => (
          <SelectItem key={type} value={type}>
            {type}
          </SelectItem>
        ))}
      </ToolbarFilter>
    </PageToolbar>
  );
}
