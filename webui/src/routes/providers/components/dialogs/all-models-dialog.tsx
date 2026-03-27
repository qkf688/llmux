import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";
import { parseUpstreamModelsFromConfig } from "@/lib/provider-models";
import type { BatchTestProgress, ModelTestResult, UpstreamStatus } from "../../types";

type Updater<T> = T | ((previous: T) => T);
type Setter<T> = (value: Updater<T>) => void;

interface AllModelsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  allModelsProvider: Provider | null;

  upstreamStatus: UpstreamStatus;
  upstreamModelsList: string[];

  allModelsList: string[];
  filteredAllModels: string[];

  allModelsSearchQuery: string;
  setAllModelsSearchQuery: (query: string) => void;

  allModelsTestResults: Record<string, ModelTestResult>;
  batchTesting: boolean;
  batchTestProgress: BatchTestProgress;
  syncingModels: boolean;
  addingModels: boolean;

  selectedAllModels: string[];
  setSelectedAllModels: Setter<string[]>;
  isAllFilteredSelected: boolean;
  toggleSelectAllModels: () => void;

  handleSyncUpstreamModels: () => void | Promise<void>;
  handleBatchTestAll: () => void | Promise<void>;
  handleBatchTestSelected: () => void | Promise<void>;
  handleCancelBatchTest: () => void;
  selectAllSuccessful: () => void;
  selectAllFailed: () => void;
  handleRemoveSelectedModels: () => void | Promise<void>;

  handleTestAllModel: (modelId: string) => void | Promise<unknown>;
  copyModelName: (modelId: string) => void;
  handleRemoveModelFromAll: (modelId: string) => void | Promise<void>;

  customModelInput: string;
  setCustomModelInput: (value: string) => void;
  handleAddCustomModels: () => void | Promise<void>;
}

export function AllModelsDialog({
  open,
  onOpenChange,
  allModelsProvider,
  upstreamStatus,
  upstreamModelsList,
  allModelsList,
  filteredAllModels,
  allModelsSearchQuery,
  setAllModelsSearchQuery,
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
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>{allModelsProvider?.Name || "当前提供商"}的全部模型</DialogTitle>
          <DialogDescription>手动维护模型缓存，可添加自定义模型或批量删除不再需要的条目。</DialogDescription>
        </DialogHeader>

        {/* 上游模型状态提示 */}
        {upstreamStatus === "loading" && (
          <div className="flex items-center gap-2 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md text-sm text-blue-800">
            <svg
              className="animate-spin h-4 w-4"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
            正在获取上游模型...
          </div>
        )}
        {upstreamStatus === "success" && (
          <div className="flex items-center gap-2 px-3 py-2 bg-green-50 border border-green-200 rounded-md text-sm text-green-800">
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
            已获取 {upstreamModelsList.length} 个上游模型
          </div>
        )}
        {upstreamStatus === "empty" && (
          <div className="flex items-center gap-2 px-3 py-2 bg-yellow-50 border border-yellow-200 rounded-md text-sm text-yellow-800">
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            上游未返回任何模型
          </div>
        )}
        {upstreamStatus === "error" && (
          <div className="flex items-center gap-2 px-3 py-2 bg-red-50 border border-red-200 rounded-md text-sm text-red-800">
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
            获取上游模型失败
          </div>
        )}

        <div className="flex flex-col gap-4 flex-1 min-h-0">
          <div className="flex flex-col gap-2 flex-1 min-h-0">
            <div className="flex flex-col gap-2 flex-shrink-0">
              {/* 第一行：标题 + 数量 + 搜索框 */}
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-semibold whitespace-nowrap">模型列表</p>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {allModelsSearchQuery ? `匹配 ${filteredAllModels.length} / ${allModelsList.length}` : `${allModelsList.length} 个`}
                  </span>
                </div>
                <Input
                  placeholder="搜索模型名称..."
                  value={allModelsSearchQuery}
                  onChange={(e) => setAllModelsSearchQuery(e.target.value)}
                  className="h-8 flex-1 min-w-0"
                />
              </div>

              {/* 第二行：测试结果统计（条件渲染）*/}
              {Object.keys(allModelsTestResults).length > 0 && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span>已测试: {Object.keys(allModelsTestResults).length}</span>
                  <span className="text-muted-foreground">|</span>
                  <span className="text-green-600">
                    成功: {Object.values(allModelsTestResults).filter((r) => r.success === true).length}
                  </span>
                  <span className="text-muted-foreground">|</span>
                  <span className="text-red-600">
                    失败: {Object.values(allModelsTestResults).filter((r) => r.success === false).length}
                  </span>
                </div>
              )}

              {/* 批量测试进度条 */}
              {batchTesting && (
                <div className="flex items-center gap-3 px-3 py-2 bg-blue-50 border border-blue-200 rounded-md">
                  <div className="flex-1">
                    <div className="flex items-center justify-between text-xs text-blue-800 mb-1">
                      <span>
                        测试进度：{batchTestProgress.completed}/{batchTestProgress.total}
                        (成功: {batchTestProgress.success}, 失败: {batchTestProgress.failed}, 进行中: {batchTestProgress.testing})
                      </span>
                      <span>{Math.round((batchTestProgress.completed / batchTestProgress.total) * 100)}%</span>
                    </div>
                    <div className="w-full bg-blue-200 rounded-full h-2">
                      <div
                        className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                        style={{ width: `${(batchTestProgress.completed / batchTestProgress.total) * 100}%` }}
                      />
                    </div>
                  </div>
                  <Button variant="outline" size="sm" onClick={handleCancelBatchTest} className="h-7 text-xs">
                    取消
                  </Button>
                </div>
              )}

              <div className="flex items-center gap-1 flex-wrap">
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="secondary"
                        size="icon"
                        className="h-8 w-8"
                        onClick={handleSyncUpstreamModels}
                        disabled={syncingModels || batchTesting || !allModelsProvider}
                      >
                        {syncingModels ? (
                          <Spinner className="h-4 w-4" />
                        ) : (
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                            />
                          </svg>
                        )}
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{syncingModels ? "同步中..." : "同步上游模型"}</TooltipContent>
                  </Tooltip>
                </TooltipProvider>

                {/* 批量测试按钮 */}
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="default"
                        size="icon"
                        className="h-8 w-8"
                        onClick={handleBatchTestAll}
                        disabled={filteredAllModels.length === 0 || batchTesting || addingModels}
                      >
                        {batchTesting ? (
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
                        onClick={handleBatchTestSelected}
                        disabled={selectedAllModels.length === 0 || batchTesting || addingModels}
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
                    <TooltipContent>批量测试选中的 {selectedAllModels.length} 个模型</TooltipContent>
                  </Tooltip>
                </TooltipProvider>

                {/* 选择成功和失败按钮 */}
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-8 w-8"
                        onClick={selectAllSuccessful}
                        disabled={
                          Object.values(allModelsTestResults).filter((r) => r.success === true).length === 0 || batchTesting
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
                        onClick={selectAllFailed}
                        disabled={
                          Object.values(allModelsTestResults).filter((r) => r.success === false).length === 0 || batchTesting
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
                <TooltipProvider>
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
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                          </svg>
                        ) : (
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                        )}
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{isAllFilteredSelected ? "取消全选" : "全选"}</TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="destructive"
                        size="icon"
                        className="h-8 w-8"
                        onClick={handleRemoveSelectedModels}
                        disabled={selectedAllModels.length === 0 || addingModels || batchTesting}
                      >
                        {addingModels ? (
                          <Spinner className="h-4 w-4" />
                        ) : (
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
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
                      {addingModels ? "删除中..." : `删除所选${selectedAllModels.length > 0 ? `（${selectedAllModels.length}）` : ""}`}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <span
                  className={`text-xs text-muted-foreground ml-1 inline-flex min-w-[64px] justify-end tabular-nums ${selectedAllModels.length > 0 ? "" : "invisible"}`}
                >
                  已选 {selectedAllModels.length} 个
                </span>
              </div>
            </div>
            <div className="border rounded-md flex-1 min-h-0 overflow-y-auto">
              {allModelsList.length === 0 ? (
                <div className="text-sm text-muted-foreground text-center py-4">暂无缓存模型</div>
              ) : filteredAllModels.length === 0 ? (
                <div className="text-sm text-muted-foreground text-center py-4">没有找到匹配的模型</div>
              ) : (
                filteredAllModels.map((model) => {
                  const checked = selectedAllModels.includes(model);
                  const upstreamModels = allModelsProvider ? parseUpstreamModelsFromConfig(allModelsProvider.Config) : [];
                  const isUpstream = upstreamModels.includes(model);
                  const testResult = allModelsTestResults[model];
                  return (
                    <div
                      key={model}
                      className={`flex items-center justify-between px-3 py-2.5 text-sm gap-2 transition-colors border-b last:border-b-0 ${checked ? "bg-blue-50/80" : "hover:bg-muted/50"}`}
                    >
                      <div className="flex items-center gap-2 min-w-0">
                        <Checkbox
                          checked={checked}
                          onCheckedChange={(value) => {
                            if (value) {
                              setSelectedAllModels((prev) => Array.from(new Set([...prev, model])));
                            } else {
                              setSelectedAllModels((prev) => prev.filter((item) => item !== model));
                            }
                          }}
                          aria-label={`选择模型 ${model}`}
                        />
                        <div className="flex items-center gap-1.5 min-w-0">
                          <span className="truncate font-mono text-xs">{model}</span>
                          <span
                            className={`flex-shrink-0 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium ${isUpstream ? "bg-blue-100 text-blue-700" : "bg-gray-100 text-gray-600"}`}
                          >
                            {isUpstream ? "上游" : "自定义"}
                          </span>
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
                                onClick={() => handleTestAllModel(model)}
                                disabled={!!testResult?.loading || batchTesting}
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
                              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => copyModelName(model)}>
                                <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
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
                        </TooltipProvider>

                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-7 w-7 text-muted-foreground hover:text-destructive"
                                onClick={() => handleRemoveModelFromAll(model)}
                                disabled={addingModels || batchTesting}
                              >
                                <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
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
                        </TooltipProvider>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
            <div className="flex items-center justify-between gap-3 flex-shrink-0">
              <Textarea
                value={customModelInput}
                onChange={(e) => setCustomModelInput(e.target.value)}
                placeholder="每行一个模型 ID，可用来自定义或补充上游未返回的模型"
                className="h-16 resize-none flex-1"
              />
              <div className="flex gap-2">
                <Button size="sm" onClick={handleAddCustomModels} disabled={addingModels || !allModelsProvider || batchTesting}>
                  {addingModels ? "提交中..." : "添加"}
                </Button>
                <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
                  关闭
                </Button>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
