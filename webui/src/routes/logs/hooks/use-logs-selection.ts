import type { ChatLog } from "@/lib/api";
import type { LogsPageState } from "@/stores/logs";

type UseLogsSelectionInput = {
  logs: ChatLog[];
  selectedIds: LogsPageState["selectedIds"];
  setSelectedIds: LogsPageState["setSelectedIds"];
  clearSelection: LogsPageState["clearSelection"];
};

export function useLogsSelection({ logs, selectedIds, setSelectedIds, clearSelection }: UseLogsSelectionInput) {
  const selectedCount = selectedIds.size;
  const isAllSelected = logs.length > 0 && selectedCount === logs.length;
  const isSomeSelected = selectedCount > 0 && selectedCount < logs.length;

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(logs.map((log) => log.ID)));
      return;
    }
    clearSelection();
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    const next = new Set(selectedIds);
    if (checked) {
      next.add(id);
    } else {
      next.delete(id);
    }
    setSelectedIds(next);
  };

  return {
    selectedCount,
    isAllSelected,
    isSomeSelected,
    handleSelectAll,
    handleSelectOne,
  };
}

