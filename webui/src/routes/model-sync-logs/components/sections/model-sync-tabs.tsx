import { cn } from "@/lib/utils";
import type { ModelSyncTab } from "../../types";

type ModelSyncTabsProps = {
  activeTab: ModelSyncTab;
  onChange: (tab: ModelSyncTab) => void;
};

export function ModelSyncTabs({ activeTab, onChange }: ModelSyncTabsProps) {
  return (
    <div className="flex gap-2 border-b flex-shrink-0">
      <button
        className={cn(
          "px-4 py-2 font-medium transition-colors",
          activeTab === "logs"
            ? "border-b-2 border-primary text-primary"
            : "text-muted-foreground hover:text-foreground"
        )}
        onClick={() => onChange("logs")}
      >
        同步日志
      </button>
      <button
        className={cn(
          "px-4 py-2 font-medium transition-colors",
          activeTab === "errors"
            ? "border-b-2 border-primary text-primary"
            : "text-muted-foreground hover:text-foreground"
        )}
        onClick={() => onChange("errors")}
      >
        最近错误
      </button>
      <button
        className={cn(
          "px-4 py-2 font-medium transition-colors",
          activeTab === "recent"
            ? "border-b-2 border-primary text-primary"
            : "text-muted-foreground hover:text-foreground"
        )}
        onClick={() => onChange("recent")}
      >
        最近新增模型
      </button>
    </div>
  );
}
