import { useMemo } from "react";
import { AnimatePresence } from "motion/react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { AllModelsDialogProps } from "./all-models-dialog-types";
import { AllModelsUpstreamStatus } from "./all-models-upstream-status";
import { AllModelsToolbar } from "./all-models-toolbar";
import { AllModelsListItem } from "./all-models-list-item";
import { AllModelsCustomAdd } from "./all-models-custom-add";

export function AllModelsDialog({
  open,
  onOpenChange,
  allModelsProvider,
  upstreamStatus,
  upstreamModelsList,
  upstreamSet,
  allModelsList,
  filteredAllModels,
  allModelsSearchQuery,
  setAllModelsSearchQuery,
  allModelsTypeFilter,
  setAllModelsTypeFilter,
  allModelsTestResults,
  batchTesting,
  batchTestProgress,
  syncingModels,
  addingModels,
  selectedAllModels,
  setSelectedAllModels,
  isAllFilteredSelected,
  toggleSelectAllModels,
  handleSyncUpstreamModels,
  handleBatchTestAll,
  handleBatchTestSelected,
  handleCancelBatchTest,
  selectAllSuccessful,
  selectAllFailed,
  handleRemoveSelectedModels,
  handleTestAllModel,
  copyModelName,
  handleRemoveModelFromAll,
  customModelInput,
  setCustomModelInput,
  handleAddCustomModels,
}: AllModelsDialogProps) {
  const selectedSet = useMemo(
    () => new Set(selectedAllModels),
    [selectedAllModels],
  );
  const testStats = useMemo(() => {
    let success = 0;
    let failed = 0;
    for (const r of Object.values(allModelsTestResults)) {
      if (r.success === true) success += 1;
      else if (r.success === false) failed += 1;
    }
    return {
      tested: Object.keys(allModelsTestResults).length,
      success,
      failed,
    };
  }, [allModelsTestResults]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>
            {allModelsProvider?.Name || "当前提供商"}的全部模型
          </DialogTitle>
          <DialogDescription>
            手动维护模型缓存，可添加自定义模型或批量删除不再需要的条目。
          </DialogDescription>
        </DialogHeader>

        <AllModelsUpstreamStatus
          status={upstreamStatus}
          upstreamCount={upstreamModelsList.length}
        />

        <div className="flex flex-col gap-4 flex-1 min-h-0">
          <div className="flex flex-col gap-2 flex-1 min-h-0">
            <AllModelsToolbar
              allModelsList={allModelsList}
              filteredAllModels={filteredAllModels}
              allModelsSearchQuery={allModelsSearchQuery}
              setAllModelsSearchQuery={setAllModelsSearchQuery}
              allModelsTypeFilter={allModelsTypeFilter}
              setAllModelsTypeFilter={setAllModelsTypeFilter}
              testStats={testStats}
              batchTesting={batchTesting}
              batchTestProgress={batchTestProgress}
              syncingModels={syncingModels}
              addingModels={addingModels}
              allModelsProvider={allModelsProvider}
              selectedAllModels={selectedAllModels}
              isAllFilteredSelected={isAllFilteredSelected}
              handleSyncUpstreamModels={handleSyncUpstreamModels}
              handleBatchTestAll={handleBatchTestAll}
              handleBatchTestSelected={handleBatchTestSelected}
              handleCancelBatchTest={handleCancelBatchTest}
              selectAllSuccessful={selectAllSuccessful}
              selectAllFailed={selectAllFailed}
              handleRemoveSelectedModels={handleRemoveSelectedModels}
              toggleSelectAllModels={toggleSelectAllModels}
            />

            <div className="border rounded-md flex-1 min-h-0 overflow-y-auto">
              {/* AnimatePresence 常驻：不能放进条件分支，否则删除到空列表时 AnimatePresence 会随分支切换整体卸载，正在 exit 的 item 被强制拔掉，退场动画不播 */}
              <AnimatePresence initial={false}>
                {filteredAllModels.map((model) => {
                  const checked = selectedSet.has(model);
                  const isUpstream = upstreamSet.has(model.toLowerCase());
                  const testResult = allModelsTestResults[model];
                  return (
                    <AllModelsListItem
                      key={model}
                      model={model}
                      checked={checked}
                      isUpstream={isUpstream}
                      testResult={testResult}
                      batchTesting={batchTesting}
                      addingModels={addingModels}
                      onToggle={(value) => {
                        if (value) {
                          setSelectedAllModels((prev) =>
                            prev.includes(model) ? prev : [...prev, model],
                          );
                        } else {
                          setSelectedAllModels((prev) =>
                            prev.filter((item) => item !== model),
                          );
                        }
                      }}
                      onTest={() => handleTestAllModel(model)}
                      onCopy={() => copyModelName(model)}
                      onRemove={() => handleRemoveModelFromAll(model)}
                    />
                  );
                })}
              </AnimatePresence>
              {allModelsList.length === 0 ? (
                <div className="text-sm text-muted-foreground text-center py-4">
                  暂无缓存模型
                </div>
              ) : filteredAllModels.length === 0 ? (
                <div className="text-sm text-muted-foreground text-center py-4">
                  没有找到匹配的模型
                </div>
              ) : null}
            </div>

            <AllModelsCustomAdd
              value={customModelInput}
              onChange={setCustomModelInput}
              onAdd={handleAddCustomModels}
              onClose={() => onOpenChange(false)}
              addingModels={addingModels}
              batchTesting={batchTesting}
              provider={allModelsProvider}
            />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
