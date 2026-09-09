import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { GroupRef, ProviderModel } from "@/lib/api";
import type { ModelTestResult } from "../../types";
import { GroupBadges, ModelTestStatusButton } from "../shared";

interface UpstreamModelsListItemProps {
  model: ProviderModel;
  checked: boolean;
  isSaved: boolean;
  testResult: ModelTestResult | undefined;
  groupTestResults: Record<string, ModelTestResult>;
  batchTesting: boolean;
  onToggle: (checked: boolean) => void;
  onTest: () => void;
  onGroupTest: (group: GroupRef) => void;
  onCopy: () => void;
}

export function UpstreamModelsListItem({
  model,
  checked,
  isSaved,
  testResult,
  groupTestResults,
  batchTesting,
  onToggle,
  onTest,
  onGroupTest,
  onCopy,
}: UpstreamModelsListItemProps) {
  // 从结果表提取本模型各分组的状态，GroupBadges 不感知键格式
  const groupStatuses: Record<number, ModelTestResult> = {};
  for (const g of model.Groups ?? []) {
    const entry = groupTestResults[`${model.id}::${g.ID}`];
    if (entry) groupStatuses[g.ID] = entry;
  }

  return (
    <div
      className={`flex items-center justify-between p-2 border rounded-lg ${
        isSaved
          ? "border-muted bg-muted/50"
          : checked
            ? "bg-info-tint border-info/30"
            : "border-border bg-background"
      }`}
    >
      <div className="flex items-center gap-3 min-w-0">
        <Checkbox
          checked={checked}
          disabled={isSaved || batchTesting}
          onCheckedChange={(value) => onToggle(!!value)}
        />
        <div className="min-w-0 flex-1">
          <div
            className={`font-medium truncate ${isSaved ? "text-muted-foreground/70" : ""}`}
          >
            {model.id}
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <GroupBadges
              groups={model.Groups}
              groupStatuses={groupStatuses}
              onGroupClick={onGroupTest}
              disabled={batchTesting}
            />
            {isSaved && (
              <span className="text-xs text-muted-foreground">已在目录</span>
            )}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-1 flex-shrink-0">
        <ModelTestStatusButton
          result={testResult}
          onTest={onTest}
          disabled={batchTesting}
        />

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              onClick={onCopy}
              className="h-7 px-2"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                strokeWidth="2"
                stroke="currentColor"
                aria-hidden="true"
                className="h-3.5 w-3.5"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                ></path>
              </svg>
            </Button>
          </TooltipTrigger>
          <TooltipContent>复制名称</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}
