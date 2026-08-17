import { PageToolbar, ToolbarSearch } from "@/components/page-toolbar";

interface ModelsToolbarProps {
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
}

/** models 筛选栏：只有名称搜索，页头职责在 ModelsHeader */
export function ModelsToolbar({ searchQuery, onSearchQueryChange }: ModelsToolbarProps) {
  return (
    <PageToolbar>
      <ToolbarSearch
        ariaLabel="搜索模型名称"
        placeholder="搜索模型名称..."
        value={searchQuery}
        onChange={onSearchQueryChange}
      />
    </PageToolbar>
  );
}
