import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ProviderModel } from "@/lib/api";
import type { BatchTestProgress, ModelTestResult } from "../../types";
import {
  BatchTestActionButtons,
  BatchTestProgressBar,
  TestStatsDisplay,
} from "../shared";

interface UpstreamModelsToolbarProps {
  modelsLoading: boolean;
  addingModels: boolean;
  providerModels: ProviderModel[];
  filteredProviderModels: ProviderModel[];
  cachedModelsCount: number;
  selectedUpstreamModels: string[];
  upstreamTestResults: Record<string, ModelTestResult>;
  upstreamBatchTesting: boolean;
  upstreamBatchTestProgress: BatchTestProgress;
  selectableModelIds: string[];
  isAllSelectableChecked: boolean;
  modelsOpenId: number | null;
  handleBatchTestUpstreamAll: () => void | Promise<void>;
  handleBatchTestUpstreamSelected: () => void | Promise<void>;
  handleCancelUpstreamBatchTest: () => void;
  selectUpstreamSuccessful: () => void;
  selectUpstreamFailed: () => void;
  toggleSelectAll: () => void;
  refreshUpstreamModels: () => void | Promise<void>;
  handleAddUpstreamToAll: () => void | Promise<void>;
  handleUpstreamSearchChange: (value: string) => void;
}

export function UpstreamModelsToolbar({
  modelsLoading,
  addingModels,
  providerModels,
  filteredProviderModels,
  cachedModelsCount,
  selectedUpstreamModels,
  upstreamTestResults,
  upstreamBatchTesting,
  upstreamBatchTestProgress,
  selectableModelIds,
  isAllSelectableChecked,
  modelsOpenId,
  handleBatchTestUpstreamAll,
  handleBatchTestUpstreamSelected,
  handleCancelUpstreamBatchTest,
  selectUpstreamSuccessful,
  selectUpstreamFailed,
  toggleSelectAll,
  refreshUpstreamModels,
  handleAddUpstreamToAll,
  handleUpstreamSearchChange,
}: UpstreamModelsToolbarProps) {
  const tested = Object.keys(upstreamTestResults).length;
  const success = Object.values(upstreamTestResults).filter(
    (r) => r.success === true,
  ).length;
  const failed = Object.values(upstreamTestResults).filter(
    (r) => r.success === false,
  ).length;

  return (
    <>
      {/* 测试结果统计 */}
      {tested > 0 && (
        <TestStatsDisplay tested={tested} success={success} failed={failed} />
      )}

      {/* 批量测试进度条 */}
      {upstreamBatchTesting && (
        <BatchTestProgressBar
          progress={upstreamBatchTestProgress}
          onCancel={handleCancelUpstreamBatchTest}
        />
      )}

      <div className="flex items-center justify-between gap-2 flex-shrink-0">
        <div className="text-sm text-muted-foreground">
          {modelsLoading
            ? "正在从上游获取..."
            : `上游返回 ${providerModels.length} 个，目录已收录 ${cachedModelsCount} 个`}
        </div>
        <div className="flex gap-1 flex-wrap">
          <BatchTestActionButtons
            onBatchTestAll={handleBatchTestUpstreamAll}
            onBatchTestSelected={handleBatchTestUpstreamSelected}
            onSelectSuccessful={selectUpstreamSuccessful}
            onSelectFailed={selectUpstreamFailed}
            batchTesting={upstreamBatchTesting}
            addingModels={addingModels}
            allCount={filteredProviderModels.length}
            selectedCount={selectedUpstreamModels.length}
            successCount={success}
            failedCount={failed}
          />

          <Button
            variant="outline"
            size="sm"
            onClick={toggleSelectAll}
            disabled={selectableModelIds.length === 0 || upstreamBatchTesting}
          >
            {isAllSelectableChecked ? "取消全选" : "全选可添加"}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={refreshUpstreamModels}
            disabled={!modelsOpenId || modelsLoading || upstreamBatchTesting}
          >
            {modelsLoading ? "刷新中" : "刷新上游"}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleAddUpstreamToAll}
            disabled={
              selectedUpstreamModels.length === 0 ||
              addingModels ||
              upstreamBatchTesting
            }
          >
            {addingModels
              ? "同步中..."
              : `添加到全部模型${selectedUpstreamModels.length > 0 ? `（${selectedUpstreamModels.length}）` : ""}`}
          </Button>
        </div>
      </div>

      {!modelsLoading && providerModels.length > 0 && (
        <div className="mb-3">
          <Input
            placeholder="搜索模型 ID"
            onChange={(e) => handleUpstreamSearchChange(e.target.value)}
            className="w-full"
          />
        </div>
      )}
    </>
  );
}
