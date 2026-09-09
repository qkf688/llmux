import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { GroupRef, ProviderModel } from "@/lib/api";
import type { BatchTestProgress, ModelTestResult } from "../../types";
import {
  BatchTestActionButtons,
  BatchTestProgressBar,
  GroupFilterSelect,
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
  availableGroups: GroupRef[];
  groupFilter: string;
  onGroupFilterChange: (value: string) => void;
  handleBatchTestUpstreamAll: () => void | Promise<void>;
  handleBatchTestUpstreamSelected: () => void | Promise<void>;
  handleCancelUpstreamBatchTest: () => void;
  selectUpstreamSuccessful: () => void;
  selectUpstreamFailed: () => void;
  toggleSelectAll: () => void;
  refreshUpstreamModels: () => void | Promise<void>;
  handleAddUpstreamToAll: () => void | Promise<void>;
  handleUpstreamSearchChange: (value: string) => void;
  clearUpstreamTestResults: () => void;
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
  availableGroups,
  groupFilter,
  onGroupFilterChange,
  handleBatchTestUpstreamAll,
  handleBatchTestUpstreamSelected,
  handleCancelUpstreamBatchTest,
  selectUpstreamSuccessful,
  selectUpstreamFailed,
  toggleSelectAll,
  refreshUpstreamModels,
  handleAddUpstreamToAll,
  handleUpstreamSearchChange,
  clearUpstreamTestResults,
}: UpstreamModelsToolbarProps) {
  // 只统计自动口径条目（::0），双写的分组条目不参与计数（与 select 同源）
  const autoEntries = Object.entries(upstreamTestResults).filter(([key]) =>
    key.endsWith("::0"),
  );
  const tested = autoEntries.length;
  const success = autoEntries.filter(([, r]) => r.success === true).length;
  const failed = autoEntries.filter(([, r]) => r.success === false).length;

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
        <div className="flex gap-1 flex-wrap items-center">
          {tested > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={clearUpstreamTestResults}
              disabled={upstreamBatchTesting}
            >
              清除结果
            </Button>
          )}
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

      {!modelsLoading && filteredProviderModels.length > 0 && (
        <div className="text-[11px] text-muted-foreground">
          批量测试将发起 {filteredProviderModels.length} 次真实计费请求
        </div>
      )}

      {!modelsLoading && providerModels.length > 0 && (
        <div className="flex items-center gap-2 mb-3">
          <Input
            placeholder="搜索模型 ID"
            onChange={(e) => handleUpstreamSearchChange(e.target.value)}
            className="flex-1"
          />
          <GroupFilterSelect
            groups={availableGroups}
            value={groupFilter}
            onChange={onGroupFilterChange}
            disabled={upstreamBatchTesting}
          />
        </div>
      )}
    </>
  );
}
