import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ModelTestResult } from "../../types";

interface ModelTestStatusButtonProps {
  result: ModelTestResult | undefined;
  onTest: () => void;
  disabled: boolean;
}

export function ModelTestStatusButton({
  result,
  onTest,
  disabled,
}: ModelTestStatusButtonProps) {
  const tooltip = result?.loading
    ? "测试中..."
    : result?.success === true
      ? "测试成功"
      : result?.success === false
        ? result.error || "测试失败"
        : "测试模型可用性";

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={onTest}
          disabled={!!result?.loading || disabled}
        >
          {result?.loading ? (
            <Spinner className="h-3.5 w-3.5" />
          ) : result?.success === true ? (
            <svg
              className="h-3.5 w-3.5 text-success"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M5 13l4 4L19 7"
              />
            </svg>
          ) : result?.success === false ? (
            <svg
              className="h-3.5 w-3.5 text-destructive"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          ) : (
            <svg
              className="h-3.5 w-3.5 text-muted-foreground"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          )}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  );
}
