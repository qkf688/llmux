import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import Loading from "@/components/loading";
import type { UpstreamModelsDialogProps } from "./upstream-models-dialog-types";
import { UpstreamModelsToolbar } from "./upstream-models-toolbar";
import { UpstreamModelsListItem } from "./upstream-models-list-item";

export function UpstreamModelsDialog({
  open,
  onOpenChange,
  providerName,
  modelsOpenId,
  modelsLoading,
  addingModels,
  providerModels,
  filteredProviderModels,
  cachedModelsCount,
  savedModelSet,
  selectedUpstreamModels,
  setSelectedUpstreamModels,
  upstreamTestResults,
  upstreamBatchTesting,
  upstreamBatchTestProgress,
  selectableModelIds,
  isAllSelectableChecked,
  toggleSelectAll,
  handleBatchTestUpstreamAll,
  handleBatchTestUpstreamSelected,
  handleCancelUpstreamBatchTest,
  selectUpstreamSuccessful,
  selectUpstreamFailed,
  refreshUpstreamModels,
  handleUpstreamSearchChange,
  handleAddUpstreamToAll,
  handleTestUpstreamModel,
  copyModelName,
}: UpstreamModelsDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{providerName} 上游模型</DialogTitle>
          <DialogDescription>
            从上游拉取的模型列表，勾选后可加入"全部模型"缓存。
          </DialogDescription>
        </DialogHeader>

        <UpstreamModelsToolbar
          modelsLoading={modelsLoading}
          addingModels={addingModels}
          providerModels={providerModels}
          filteredProviderModels={filteredProviderModels}
          cachedModelsCount={cachedModelsCount}
          selectedUpstreamModels={selectedUpstreamModels}
          upstreamTestResults={upstreamTestResults}
          upstreamBatchTesting={upstreamBatchTesting}
          upstreamBatchTestProgress={upstreamBatchTestProgress}
          selectableModelIds={selectableModelIds}
          isAllSelectableChecked={isAllSelectableChecked}
          modelsOpenId={modelsOpenId}
          handleBatchTestUpstreamAll={handleBatchTestUpstreamAll}
          handleBatchTestUpstreamSelected={handleBatchTestUpstreamSelected}
          handleCancelUpstreamBatchTest={handleCancelUpstreamBatchTest}
          selectUpstreamSuccessful={selectUpstreamSuccessful}
          selectUpstreamFailed={selectUpstreamFailed}
          toggleSelectAll={toggleSelectAll}
          refreshUpstreamModels={refreshUpstreamModels}
          handleAddUpstreamToAll={handleAddUpstreamToAll}
          handleUpstreamSearchChange={handleUpstreamSearchChange}
        />

        <DialogBody className="space-y-2">
          {modelsLoading ? (
            <Loading message="加载模型列表" />
          ) : filteredProviderModels.length === 0 ? (
            <div className="text-center text-muted-foreground py-8">
              {providerModels.length === 0
                ? "暂无模型数据"
                : "未找到匹配的模型"}
            </div>
          ) : (
            filteredProviderModels.map((model) => {
              const isSaved = savedModelSet.has(model.id.toLowerCase());
              const checked = selectedUpstreamModels.includes(model.id);
              const testResult = upstreamTestResults[model.id];
              return (
                <UpstreamModelsListItem
                  key={model.id}
                  model={model}
                  checked={checked}
                  isSaved={isSaved}
                  testResult={testResult}
                  batchTesting={upstreamBatchTesting}
                  onToggle={(value) => {
                    if (value) {
                      setSelectedUpstreamModels((prev) =>
                        Array.from(new Set([...prev, model.id])),
                      );
                    } else {
                      setSelectedUpstreamModels((prev) =>
                        prev.filter((item) => item !== model.id),
                      );
                    }
                  }}
                  onTest={() => handleTestUpstreamModel(model.id)}
                  onCopy={() => copyModelName(model.id)}
                />
              );
            })
          )}
        </DialogBody>

        <DialogFooter>
          <Button onClick={() => onOpenChange(false)}>关闭</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
