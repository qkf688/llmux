import { AnimatedListItem } from "@/components/ui/animated-list-item";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ModelTestResult } from "../../types";
import { ModelTestStatusButton } from "../shared";

interface AllModelsListItemProps {
  model: string;
  checked: boolean;
  isUpstream: boolean;
  testResult: ModelTestResult | undefined;
  batchTesting: boolean;
  addingModels: boolean;
  onToggle: (checked: boolean) => void;
  onTest: () => void;
  onCopy: () => void;
  onRemove: () => void;
}

export function AllModelsListItem({
  model,
  checked,
  isUpstream,
  testResult,
  batchTesting,
  addingModels,
  onToggle,
  onTest,
  onCopy,
  onRemove,
}: AllModelsListItemProps) {
  return (
    <AnimatedListItem
      className={`flex items-center justify-between px-3 py-2.5 text-sm gap-2 transition-colors border-b last:border-b-0 ${checked ? "bg-info-tint" : "hover:bg-muted/50"}`}
    >
      <div className="flex items-center gap-2 min-w-0">
        <Checkbox
          checked={checked}
          onCheckedChange={(value) => onToggle(!!value)}
          aria-label={`选择模型 ${model}`}
        />
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="truncate font-mono text-xs">{model}</span>
          <span
            className={`flex-shrink-0 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium ${isUpstream ? "bg-info-tint text-info-foreground" : "bg-muted text-muted-foreground"}`}
          >
            {isUpstream ? "上游" : "自定义"}
          </span>
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
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={onCopy}
            >
              <svg
                className="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                />
              </svg>
            </Button>
          </TooltipTrigger>
          <TooltipContent>复制名称</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              onClick={onRemove}
              disabled={addingModels || batchTesting}
            >
              <svg
                className="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </Button>
          </TooltipTrigger>
          <TooltipContent>移除</TooltipContent>
        </Tooltip>
      </div>
    </AnimatedListItem>
  );
}
