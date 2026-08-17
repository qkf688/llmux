import { PageToolbar } from "@/components/page-toolbar";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { CheckSquare2, Trash2 } from "lucide-react";

type LogsActionsBarProps = {
  logsCount: number;
  selectedCount: number;
  allSelected: boolean;
  showUnchanged: boolean;
  onToggleSelectAll: () => void;
  onDeleteSelected: () => void;
  onOpenClearDialog: () => void;
  onShowUnchangedChange: (checked: boolean) => void;
};

export function LogsActionsBar({
  logsCount,
  selectedCount,
  allSelected,
  showUnchanged,
  onToggleSelectAll,
  onDeleteSelected,
  onOpenClearDialog,
  onShowUnchangedChange,
}: LogsActionsBarProps) {
  return (
    <PageToolbar className="flex-row items-center justify-between">
      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
          onClick={onToggleSelectAll}
          disabled={logsCount === 0}
        >
          <CheckSquare2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 mr-1" />
          <span className="hidden sm:inline">{allSelected && logsCount > 0 ? "取消全选" : "全选"}</span>
          <span className="sm:hidden">{allSelected && logsCount > 0 ? "取消" : "全选"}</span>
        </Button>
        <Button
          variant="destructive"
          size="sm"
          className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
          onClick={onDeleteSelected}
          disabled={selectedCount === 0}
        >
          <Trash2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 mr-1" />
          <span className="hidden sm:inline">删除所选 ({selectedCount})</span>
          <span className="sm:hidden">删除({selectedCount})</span>
        </Button>
        <Button
          variant="destructive"
          size="sm"
          className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
          onClick={onOpenClearDialog}
        >
          <Trash2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 mr-1" />
          <span className="hidden sm:inline">清空全部</span>
          <span className="sm:hidden">清空</span>
        </Button>
      </div>
      <div className="flex items-center gap-2">
        <Switch
          id="show-unchanged"
          checked={showUnchanged}
          onCheckedChange={onShowUnchangedChange}
          className="sm:h-5 sm:w-9 h-4 w-7"
        />
        <label htmlFor="show-unchanged" className="sm:text-sm text-xs cursor-pointer">
          显示全部
        </label>
      </div>
    </PageToolbar>
  );
}
