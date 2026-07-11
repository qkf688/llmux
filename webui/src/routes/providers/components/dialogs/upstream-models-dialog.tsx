import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import Loading from "@/components/loading";
import type { ProviderModel } from "@/lib/api";
import type { Updater } from "@/stores/core/updater";
import type { BatchTestProgress, ModelTestResult } from "../../types";

type Setter<T> = (value: Updater<T>) => void;

interface UpstreamModelsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providerName: string | undefined;

  modelsOpenId: number | null;
  modelsLoading: boolean;
  addingModels: boolean;

  providerModels: ProviderModel[];
  filteredProviderModels: ProviderModel[];

  cachedModelsCount: number;
  savedModelSet: Set<string>;

  selectedUpstreamModels: string[];
  setSelectedUpstreamModels: Setter<string[]>;

  upstreamTestResults: Record<string, ModelTestResult>;
  upstreamBatchTesting: boolean;
  upstreamBatchTestProgress: BatchTestProgress;

  selectableModelIds: string[];
  isAllSelectableChecked: boolean;
  toggleSelectAll: () => void;

  handleBatchTestUpstreamAll: () => void | Promise<void>;
  handleBatchTestUpstreamSelected: () => void | Promise<void>;
  handleCancelUpstreamBatchTest: () => void;
  selectUpstreamSuccessful: () => void;
  selectUpstreamFailed: () => void;

  refreshUpstreamModels: () => void | Promise<void>;
  handleUpstreamSearchChange: (value: string) => void;
  handleAddUpstreamToAll: () => void | Promise<void>;

  handleTestUpstreamModel: (modelId: string) => void | Promise<unknown>;
  copyModelName: (modelId: string) => void;
}

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
      <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>{providerName} 上游模型</DialogTitle>
          <DialogDescription>从上游拉取的模型列表，勾选后可加入"全部模型"缓存。</DialogDescription>
        </DialogHeader>

        {/* 测试结果统计 */}
        {Object.keys(upstreamTestResults).length > 0 && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground flex-shrink-0">
            <span>已测试: {Object.keys(upstreamTestResults).length}</span>
            <span className="text-muted-foreground">|</span>
            <span className="text-green-600">
              成功: {Object.values(upstreamTestResults).filter((r) => r.success === true).length}
            </span>
            <span className="text-muted-foreground">|</span>
            <span className="text-red-600">
              失败: {Object.values(upstreamTestResults).filter((r) => r.success === false).length}
            </span>
          </div>
        )}

        {/* 批量测试进度条 */}
        {upstreamBatchTesting && (
          <div className="flex items-center gap-3 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md flex-shrink-0">
            <div className="flex-1">
              <div className="flex items-center justify-between text-xs text-blue-800 mb-1">
                <span>
                  测试进度：{upstreamBatchTestProgress.completed}/{upstreamBatchTestProgress.total}
                  (成功: {upstreamBatchTestProgress.success}, 失败: {upstreamBatchTestProgress.failed}, 进行中: {upstreamBatchTestProgress.testing})
                </span>
                <span>{Math.round((upstreamBatchTestProgress.completed / upstreamBatchTestProgress.total) * 100)}%</span>
              </div>
              <div className="w-full bg-blue-200 rounded-full h-2">
                <div
                  className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${(upstreamBatchTestProgress.completed / upstreamBatchTestProgress.total) * 100}%` }}
                />
              </div>
            </div>
            <Button variant="outline" size="sm" onClick={handleCancelUpstreamBatchTest} className="h-7 text-xs">
              取消
            </Button>
          </div>
        )}

        <div className="flex items-center justify-between gap-2 flex-shrink-0">
          <div className="text-sm text-muted-foreground">
            {modelsLoading ? "正在从上游获取..." : `上游返回 ${providerModels.length} 个，已缓存 ${cachedModelsCount} 个`}
          </div>
          <div className="flex gap-1 flex-wrap">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="default"
                    size="icon"
                    className="h-8 w-8"
                    onClick={handleBatchTestUpstreamAll}
                    disabled={filteredProviderModels.length === 0 || upstreamBatchTesting || addingModels}
                  >
                    {upstreamBatchTesting ? (
                      <Spinner className="h-4 w-4" />
                    ) : (
                      <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
                        />
                        <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>批量测试所有模型</TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="secondary"
                    size="icon"
                    className="h-8 w-8"
                    onClick={handleBatchTestUpstreamSelected}
                    disabled={selectedUpstreamModels.length === 0 || upstreamBatchTesting || addingModels}
                  >
                    <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
                      />
                    </svg>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>批量测试选中的 {selectedUpstreamModels.length} 个模型</TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    className="h-8 w-8"
                    onClick={selectUpstreamSuccessful}
                    disabled={
                      Object.values(upstreamTestResults).filter((r) => r.success === true).length === 0 || upstreamBatchTesting
                    }
                  >
                    <svg className="h-4 w-4 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>选择测试成功的模型</TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    className="h-8 w-8"
                    onClick={selectUpstreamFailed}
                    disabled={
                      Object.values(upstreamTestResults).filter((r) => r.success === false).length === 0 || upstreamBatchTesting
                    }
                  >
                    <svg className="h-4 w-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>选择测试失败的模型</TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <Button variant="outline" size="sm" onClick={toggleSelectAll} disabled={selectableModelIds.length === 0 || upstreamBatchTesting}>
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
              disabled={selectedUpstreamModels.length === 0 || addingModels || upstreamBatchTesting}
            >
              {addingModels ? "同步中..." : `添加到全部模型${selectedUpstreamModels.length > 0 ? `（${selectedUpstreamModels.length}）` : ""}`}
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

        {modelsLoading ? (
          <Loading message="加载模型列表" />
        ) : (
          <div className="max-h-96 overflow-y-auto space-y-2 flex-1 min-h-0">
            {filteredProviderModels.length === 0 ? (
              <div className="text-center text-gray-500 py-8">
                {providerModels.length === 0 ? "暂无模型数据" : "未找到匹配的模型"}
              </div>
            ) : (
              (() => {
                return filteredProviderModels.map((model) => {
                  const isSaved = savedModelSet.has(model.id.toLowerCase());
                  const checked = selectedUpstreamModels.includes(model.id);
                  const testResult = upstreamTestResults[model.id];
                  return (
                    <div
                      key={model.id}
                      className={`flex items-center justify-between p-2 border rounded-lg ${
                        isSaved
                          ? "border-gray-300 bg-gray-50/50"
                          : checked
                            ? "bg-blue-50/80 border-blue-200"
                            : "border-border bg-background"
                      }`}
                    >
                      <div className="flex items-center gap-3 min-w-0">
                        <Checkbox
                          checked={checked}
                          disabled={isSaved || upstreamBatchTesting}
                          onCheckedChange={(value) => {
                            if (value) {
                              setSelectedUpstreamModels((prev) => Array.from(new Set([...prev, model.id])));
                            } else {
                              setSelectedUpstreamModels((prev) => prev.filter((item) => item !== model.id));
                            }
                          }}
                        />
                        <div className="min-w-0 flex-1">
                          <div className={`font-medium truncate ${isSaved ? "text-muted-foreground/70" : ""}`}>{model.id}</div>
                          {isSaved && <span className="text-xs text-muted-foreground">已缓存</span>}
                        </div>
                      </div>
                      <div className="flex items-center gap-1 flex-shrink-0">
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-7 w-7"
                                onClick={() => handleTestUpstreamModel(model.id)}
                                disabled={!!testResult?.loading || upstreamBatchTesting}
                              >
                                {testResult?.loading ? (
                                  <Spinner className="h-3.5 w-3.5" />
                                ) : testResult?.success === true ? (
                                  <svg className="h-3.5 w-3.5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                                  </svg>
                                ) : testResult?.success === false ? (
                                  <svg className="h-3.5 w-3.5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                  </svg>
                                ) : (
                                  <svg className="h-3.5 w-3.5 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                  </svg>
                                )}
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>
                              {testResult?.loading
                                ? "测试中..."
                                : testResult?.success === true
                                  ? "测试成功"
                                  : testResult?.success === false
                                    ? testResult.error || "测试失败"
                                    : "测试模型可用性"}
                            </TooltipContent>
                          </Tooltip>
                        </TooltipProvider>

                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button variant="outline" size="sm" onClick={() => copyModelName(model.id)} className="h-7 px-2">
                                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" aria-hidden="true" className="h-3.5 w-3.5"><path strokeLinecap="round" strokeLinejoin="round" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path></svg>
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>复制名称</TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                      </div>
                    </div>
                  );
                });
              })()
            )}
          </div>
        )}

        <DialogFooter>
          <Button onClick={() => onOpenChange(false)}>关闭</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

