import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface BatchTestActionButtonsProps {
  onBatchTestAll: () => void | Promise<void>;
  onBatchTestSelected: () => void | Promise<void>;
  onSelectSuccessful: () => void;
  onSelectFailed: () => void;
  batchTesting: boolean;
  addingModels: boolean;
  allCount: number;
  selectedCount: number;
  successCount: number;
  failedCount: number;
}

export function BatchTestActionButtons({
  onBatchTestAll,
  onBatchTestSelected,
  onSelectSuccessful,
  onSelectFailed,
  batchTesting,
  addingModels,
  allCount,
  selectedCount,
  successCount,
  failedCount,
}: BatchTestActionButtonsProps) {
  return (
    <>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="default"
            size="icon"
            className="h-8 w-8"
            onClick={onBatchTestAll}
            disabled={allCount === 0 || batchTesting || addingModels}
          >
            {batchTesting ? (
              <Spinner className="h-4 w-4" />
            ) : (
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent>批量测试所有模型</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="secondary"
            size="icon"
            className="h-8 w-8"
            onClick={onBatchTestSelected}
            disabled={selectedCount === 0 || batchTesting || addingModels}
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
              />
            </svg>
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          批量测试选中的 {selectedCount} 个模型
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={onSelectSuccessful}
            disabled={successCount === 0 || batchTesting}
          >
            <svg
              className="h-4 w-4 text-success"
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
          </Button>
        </TooltipTrigger>
        <TooltipContent>选择测试成功的模型</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={onSelectFailed}
            disabled={failedCount === 0 || batchTesting}
          >
            <svg
              className="h-4 w-4 text-destructive"
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
          </Button>
        </TooltipTrigger>
        <TooltipContent>选择测试失败的模型</TooltipContent>
      </Tooltip>
    </>
  );
}
