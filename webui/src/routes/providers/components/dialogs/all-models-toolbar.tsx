import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";
import type {
  AllModelsTypeFilter,
  BatchTestProgress,
} from "../../types";
import {
  BatchTestActionButtons,
  BatchTestProgressBar,
  TestStatsDisplay,
} from "../shared";
import { FILTER_OPTIONS } from "./all-models-dialog-types";

interface AllModelsToolbarProps {
  allModelsList: string[];
  filteredAllModels: string[];
  allModelsSearchQuery: string;
  setAllModelsSearchQuery: (query: string) => void;
  allModelsTypeFilter: AllModelsTypeFilter;
  setAllModelsTypeFilter: (value: AllModelsTypeFilter) => void;
  testStats: { tested: number; success: number; failed: number };
  batchTesting: boolean;
  batchTestProgress: BatchTestProgress;
  syncingModels: boolean;
  addingModels: boolean;
  allModelsProvider: Provider | null;
  selectedAllModels: string[];
  isAllFilteredSelected: boolean;
  handleSyncUpstreamModels: () => void | Promise<void>;
  handleBatchTestAll: () => void | Promise<void>;
  handleBatchTestSelected: () => void | Promise<void>;
  handleCancelBatchTest: () => void;
  selectAllSuccessful: () => void;
  selectAllFailed: () => void;
  handleRemoveSelectedModels: () => void | Promise<void>;
  toggleSelectAllModels: () => void;
}

export function AllModelsToolbar({
  allModelsList,
  filteredAllModels,
  allModelsSearchQuery,
  setAllModelsSearchQuery,
  allModelsTypeFilter,
  setAllModelsTypeFilter,
  testStats,
  batchTesting,
  batchTestProgress,
  syncingModels,
  addingModels,
  allModelsProvider,
  selectedAllModels,
  isAllFilteredSelected,
  handleSyncUpstreamModels,
  handleBatchTestAll,
  handleBatchTestSelected,
  handleCancelBatchTest,
  selectAllSuccessful,
  selectAllFailed,
  handleRemoveSelectedModels,
  toggleSelectAllModels,
}: AllModelsToolbarProps) {
  return (
    <div className="flex flex-col gap-2 flex-shrink-0">
      {/* 第一行：标题 + 数量 + 搜索框 */}
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold whitespace-nowrap">
            模型列表
          </p>
          <span className="text-xs text-muted-foreground whitespace-nowrap">
            {allModelsSearchQuery.trim() !== "" ||
            allModelsTypeFilter !== "all"
              ? `匹配 ${filteredAllModels.length} / ${allModelsList.length}`
              : `${allModelsList.length} 个`}
          </span>
        </div>
        <Input
          placeholder="搜索模型名称..."
          value={allModelsSearchQuery}
          onChange={(e) => setAllModelsSearchQuery(e.target.value)}
          className="h-8 flex-1 min-w-0"
        />
      </div>

      {/* 类型筛选 */}
      <div className="flex items-center gap-1">
        <span className="text-xs text-muted-foreground mr-1">筛选：</span>
        <div
          className="inline-flex rounded-md border border-input bg-background"
          role="radiogroup"
          aria-label="模型类型筛选"
        >
          {FILTER_OPTIONS.map(({ key, label }) => {
            const checked = allModelsTypeFilter === key;
            return (
              <button
                key={key}
                type="button"
                role="radio"
                aria-checked={checked}
                onClick={() => setAllModelsTypeFilter(key)}
                className={`px-2.5 py-1 text-xs font-medium transition-colors first:rounded-l-md last:rounded-r-md border-r border-input last:border-r-0 ${
                  checked
                    ? "bg-primary text-primary-foreground"
                    : "bg-background text-muted-foreground hover:bg-muted"
                }`}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>

      {/* 测试结果统计 */}
      {testStats.tested > 0 && (
        <TestStatsDisplay
          tested={testStats.tested}
          success={testStats.success}
          failed={testStats.failed}
        />
      )}

      {/* 批量测试进度条 */}
      {batchTesting && (
        <BatchTestProgressBar
          progress={batchTestProgress}
          onCancel={handleCancelBatchTest}
        />
      )}

      <div className="flex items-center gap-1 flex-wrap">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="secondary"
              size="icon"
              className="h-8 w-8"
              onClick={handleSyncUpstreamModels}
              disabled={
                syncingModels || batchTesting || !allModelsProvider
              }
            >
              {syncingModels ? (
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
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                </svg>
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            {syncingModels ? "同步中..." : "同步上游模型"}
          </TooltipContent>
        </Tooltip>

        <BatchTestActionButtons
          onBatchTestAll={handleBatchTestAll}
          onBatchTestSelected={handleBatchTestSelected}
          onSelectSuccessful={selectAllSuccessful}
          onSelectFailed={selectAllFailed}
          batchTesting={batchTesting}
          addingModels={addingModels}
          allCount={filteredAllModels.length}
          selectedCount={selectedAllModels.length}
          successCount={testStats.success}
          failedCount={testStats.failed}
        />

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={toggleSelectAllModels}
              disabled={filteredAllModels.length === 0 || batchTesting}
            >
              {isAllFilteredSelected ? (
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
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
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
                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            {isAllFilteredSelected ? "取消全选" : "全选"}
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="destructive"
              size="icon"
              className="h-8 w-8"
              onClick={handleRemoveSelectedModels}
              disabled={
                selectedAllModels.length === 0 ||
                addingModels ||
                batchTesting
              }
            >
              {addingModels ? (
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
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            {addingModels
              ? "删除中..."
              : `删除所选${selectedAllModels.length > 0 ? `（${selectedAllModels.length}）` : ""}（仅自定义模型可删）`}
          </TooltipContent>
        </Tooltip>
        <span
          className={`text-xs text-muted-foreground ml-1 inline-flex min-w-[64px] justify-end tabular-nums ${selectedAllModels.length > 0 ? "" : "invisible"}`}
        >
          已选 {selectedAllModels.length} 个
        </span>
      </div>
    </div>
  );
}
