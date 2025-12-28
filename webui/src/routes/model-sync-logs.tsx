import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Switch } from "@/components/ui/switch";
import { getModelSyncLogs, deleteModelSyncLogs, clearModelSyncLogs, syncAllProviderModels, getModelSyncStats } from "@/lib/api";
import type { ModelSyncLog, ModelSyncStats } from "@/lib/api";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { RefreshCw, Clock, CheckCircle, AlertTriangle, Layers, XCircle, MinusCircle } from "lucide-react";

export default function ModelSyncLogsPage() {
  const [logs, setLogs] = useState<ModelSyncLog[]>([]);
  const [stats, setStats] = useState<ModelSyncStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [statsLoading, setStatsLoading] = useState(true);
  const [selectedLogs, setSelectedLogs] = useState<number[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [detailLog, setDetailLog] = useState<ModelSyncLog | null>(null);
  const [showClearDialog, setShowClearDialog] = useState(false);
  const [showUnchanged, setShowUnchanged] = useState(false);

  useEffect(() => {
    fetchStats();
    fetchLogs();
  }, [page, showUnchanged]);

  const fetchStats = async () => {
    try {
      setStatsLoading(true);
      const data = await getModelSyncStats();
      setStats(data);
    } catch (err) {
      console.error("加载统计信息失败:", err);
    } finally {
      setStatsLoading(false);
    }
  };

  const fetchLogs = async () => {
    try {
      setLoading(true);
      const data = await getModelSyncLogs({ page, page_size: 20, show_unchanged: showUnchanged });
      setLogs(data.data);
      setTotalPages(data.pagination.total_pages);
    } catch (err) {
      toast.error("加载日志失败: " + (err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleSyncNow = async () => {
    try {
      setSyncing(true);
      await syncAllProviderModels();
      toast.success("同步已开始，请稍后刷新查看结果");
      // 延迟刷新统计数据和日志
      setTimeout(() => {
        fetchStats();
        fetchLogs();
      }, 2000);
    } catch (err) {
      toast.error("同步失败: " + (err as Error).message);
    } finally {
      setSyncing(false);
    }
  };

  const handleDelete = async () => {
    if (selectedLogs.length === 0) return;
    try {
      await deleteModelSyncLogs(selectedLogs);
      toast.success(`已删除 ${selectedLogs.length} 条日志`);
      setSelectedLogs([]);
      fetchLogs();
    } catch (err) {
      toast.error("删除失败: " + (err as Error).message);
    }
  };

  const handleClear = async () => {
    try {
      await clearModelSyncLogs();
      toast.success("已清空所有日志");
      setShowClearDialog(false);
      fetchLogs();
    } catch (err) {
      toast.error("清空失败: " + (err as Error).message);
    }
  };

  const toggleSelectAll = () => {
    if (selectedLogs.length === logs.length) {
      setSelectedLogs([]);
    } else {
      setSelectedLogs(logs.map(log => log.ID));
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "-";
    return new Date(dateStr).toLocaleString('zh-CN');
  };

  const renderStatusBadge = (status: string) => {
    switch (status) {
      case "success":
        return (
          <div className="flex items-center gap-1 text-green-600">
            <CheckCircle className="h-4 w-4" />
            <span className="text-sm font-medium">成功</span>
          </div>
        );
      case "unchanged":
        return (
          <div className="flex items-center gap-1 text-blue-600">
            <MinusCircle className="h-4 w-4" />
            <span className="text-sm font-medium">无变化</span>
          </div>
        );
      case "error":
        return (
          <div className="flex items-center gap-1 text-red-600">
            <XCircle className="h-4 w-4" />
            <span className="text-sm font-medium">错误</span>
          </div>
        );
      default:
        return <span className="text-sm text-muted-foreground">未知</span>;
    }
  };

  // 解析错误信息，提取状态码和响应体
  const parseErrorMessage = (errorMsg: string) => {
    // 格式: "status code: 401, response: {...}"
    const statusCodeMatch = errorMsg.match(/status code:\s*(\d+)/);
    const responseMatch = errorMsg.match(/response:\s*(.+)$/);
    
    let statusCode = null;
    let responseBody = null;
    let formattedResponse = null;

    if (statusCodeMatch) {
      statusCode = statusCodeMatch[1];
    }

    if (responseMatch) {
      responseBody = responseMatch[1];
      // 尝试解析JSON并格式化
      try {
        const jsonObj = JSON.parse(responseBody);
        formattedResponse = JSON.stringify(jsonObj, null, 2);
      } catch {
        // 如果不是JSON，直接使用原始文本
        formattedResponse = responseBody;
      }
    }

    return {
      statusCode,
      responseBody: formattedResponse || responseBody,
      originalError: errorMsg
    };
  };

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      {/* 标题栏 */}
      <div className="flex items-center justify-between flex-shrink-0">
        <div>
          <h2 className="text-2xl font-bold">模型同步日志</h2>
          <p className="text-sm text-muted-foreground">查看上游模型同步记录</p>
        </div>
        <Button
          variant="default"
          size="sm"
          onClick={handleSyncNow}
          disabled={syncing}
        >
          {syncing ? (
            <>
              <RefreshCw className="h-4 w-4 mr-1 animate-spin" />
              同步中...
            </>
          ) : (
            <>
              <RefreshCw className="h-4 w-4 mr-1" />
              立即同步
            </>
          )}
        </Button>
      </div>

      {/* 统计信息 - 紧凑版 */}
      <div className="flex flex-wrap items-center gap-4 text-sm flex-shrink-0">
        {statsLoading ? (
          <div className="h-6 w-48 bg-muted animate-pulse rounded" />
        ) : (
          <div className="flex items-center gap-2">
            <Clock className="h-4 w-4 text-muted-foreground" />
            <span>
              上次: {formatDate(stats?.last_sync_at)}
              {stats?.sync_enabled && stats?.next_sync_at && (
                <> · 下次: {formatDate(stats.next_sync_at)}</>
              )}
            </span>
            {stats?.sync_enabled ? (
              <span className="text-green-600">自动 {stats.sync_interval}h</span>
            ) : (
              <span className="text-muted-foreground">手动</span>
            )}
          </div>
        )}
        
        {statsLoading ? (
          <div className="h-6 w-24 bg-muted animate-pulse rounded" />
        ) : (
          <div className="flex items-center gap-1">
            <Layers className="h-4 w-4 text-muted-foreground" />
            <span className="font-medium">{stats?.total_providers || 0}</span>
            <span className="text-muted-foreground">提供商</span>
          </div>
        )}

        {statsLoading ? (
          <div className="h-6 w-16 bg-muted animate-pulse rounded" />
        ) : (
          <div className="flex items-center gap-1">
            <CheckCircle className="h-4 w-4 text-green-500" />
            <span className="font-medium text-green-600">{stats?.providers_with_updates || 0}</span>
            <span className="text-muted-foreground">更新</span>
          </div>
        )}

        {statsLoading ? (
          <div className="h-6 w-16 bg-muted animate-pulse rounded" />
        ) : (
          <div className="flex items-center gap-1">
            <CheckCircle className="h-4 w-4 text-blue-500" />
            <span className="font-medium text-blue-600">{stats?.providers_unchanged || 0}</span>
            <span className="text-muted-foreground">无变</span>
          </div>
        )}

        {statsLoading ? (
          <div className="h-6 w-16 bg-muted animate-pulse rounded" />
        ) : (
          <div className="flex items-center gap-1">
            <AlertTriangle className="h-4 w-4 text-orange-500" />
            <span className="font-medium text-orange-600">{stats?.providers_with_errors || 0}</span>
            <span className="text-muted-foreground">错误</span>
          </div>
        )}
      </div>

      {/* 操作栏 */}
      <div className="flex items-center justify-between flex-shrink-0">
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={toggleSelectAll}
            disabled={logs.length === 0}
          >
            {selectedLogs.length === logs.length && logs.length > 0 ? "取消全选" : "全选"}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={handleDelete}
            disabled={selectedLogs.length === 0}
          >
            删除所选 ({selectedLogs.length})
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setShowClearDialog(true)}
          >
            清空全部
          </Button>
        </div>
        <div className="flex items-center gap-2">
          <Switch
            id="show-unchanged"
            checked={showUnchanged}
            onCheckedChange={setShowUnchanged}
          />
          <label htmlFor="show-unchanged" className="text-sm cursor-pointer">
            显示全部
          </label>
        </div>
      </div>

      <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading message="加载日志" />
          </div>
        ) : logs.length === 0 ? (
          <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
            暂无同步日志
          </div>
        ) : (
          <div className="h-full overflow-auto">
            <Table>
              <TableHeader className="sticky top-0 bg-secondary/80">
                <TableRow>
                  <TableHead className="w-12">
                    <Checkbox
                      checked={selectedLogs.length === logs.length && logs.length > 0}
                      onCheckedChange={toggleSelectAll}
                    />
                  </TableHead>
                  <TableHead>提供商</TableHead>
                  <TableHead>同步时间</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>变化</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => (
                  <TableRow key={log.ID}>
                    <TableCell>
                      <Checkbox
                        checked={selectedLogs.includes(log.ID)}
                        onCheckedChange={(checked) => {
                          if (checked) {
                            setSelectedLogs([...selectedLogs, log.ID]);
                          } else {
                            setSelectedLogs(selectedLogs.filter(id => id !== log.ID));
                          }
                        }}
                      />
                    </TableCell>
                    <TableCell className="font-medium">{log.ProviderName}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDate(log.SyncedAt)}
                    </TableCell>
                    <TableCell>
                      {renderStatusBadge(log.Status)}
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        {log.AddedCount > 0 && (
                          <span className="text-green-600">+{log.AddedCount}</span>
                        )}
                        {log.RemovedCount > 0 && (
                          <span className="text-red-600">-{log.RemovedCount}</span>
                        )}
                        {log.AddedCount === 0 && log.RemovedCount === 0 && log.Status !== "error" && (
                          <span className="text-muted-foreground text-sm">-</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setDetailLog(log)}
                      >
                        查看详情
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 flex-shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page === 1}
          >
            上一页
          </Button>
          <span className="text-sm">
            第 {page} / {totalPages} 页
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage(p => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
          >
            下一页
          </Button>
        </div>
      )}

      <Dialog open={!!detailLog} onOpenChange={() => setDetailLog(null)}>
        <DialogContent className="max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>同步详情</DialogTitle>
            <DialogDescription>
              {detailLog?.ProviderName} - {detailLog && formatDate(detailLog.SyncedAt)}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 overflow-y-auto flex-1">
            {/* 状态信息 */}
            {detailLog && (
              <div className="flex items-center gap-2 p-3 bg-muted rounded-md">
                <span className="text-sm text-muted-foreground">状态:</span>
                {renderStatusBadge(detailLog.Status)}
              </div>
            )}

            {/* 错误信息 */}
            {detailLog && detailLog.Status === "error" && detailLog.Error && (() => {
              const parsedError = parseErrorMessage(detailLog.Error);
              return (
                <div className="p-3 bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-900 rounded-md">
                  <h3 className="font-semibold text-red-600 mb-2 flex items-center gap-2">
                    <XCircle className="h-4 w-4" />
                    错误信息
                  </h3>
                  
                  {/* 状态码 */}
                  {parsedError.statusCode && (
                    <div className="mb-2">
                      <span className="text-sm font-medium text-red-700 dark:text-red-400">
                        HTTP状态码: <span className="font-mono">{parsedError.statusCode}</span>
                      </span>
                    </div>
                  )}
                  
                  {/* 响应体 */}
                  {parsedError.responseBody && (
                    <div className="mb-2">
                      <span className="text-sm font-medium text-red-700 dark:text-red-400 block mb-1">
                        响应内容:
                      </span>
                      <pre className="text-xs text-red-700 dark:text-red-400 bg-red-100 dark:bg-red-950/40 p-2 rounded overflow-x-auto">
                        {parsedError.responseBody}
                      </pre>
                    </div>
                  )}
                  
                  {/* 如果没有解析出状态码和响应体，显示原始错误 */}
                  {!parsedError.statusCode && !parsedError.responseBody && (
                    <div className="text-sm text-red-700 dark:text-red-400 whitespace-pre-wrap break-words">
                      {parsedError.originalError}
                    </div>
                  )}
                </div>
              );
            })()}

            {/* 新增模型 */}
            {detailLog && detailLog.AddedCount > 0 && (
              <div>
                <h3 className="font-semibold text-green-600 mb-2">新增模型 ({detailLog.AddedCount})</h3>
                <div className="max-h-40 overflow-y-auto border rounded p-2 space-y-1">
                  {detailLog.AddedModels.map((model, idx) => (
                    <div key={idx} className="text-sm">{model}</div>
                  ))}
                </div>
              </div>
            )}

            {/* 删除模型 */}
            {detailLog && detailLog.RemovedCount > 0 && (
              <div>
                <h3 className="font-semibold text-red-600 mb-2">删除模型 ({detailLog.RemovedCount})</h3>
                <div className="max-h-40 overflow-y-auto border rounded p-2 space-y-1">
                  {detailLog.RemovedModels.map((model, idx) => (
                    <div key={idx} className="text-sm">{model}</div>
                  ))}
                </div>
              </div>
            )}

            {/* 无变化提示 */}
            {detailLog && detailLog.Status === "unchanged" && (
              <div className="p-3 bg-blue-50 dark:bg-blue-950/20 border border-blue-200 dark:border-blue-900 rounded-md">
                <p className="text-sm text-blue-700 dark:text-blue-400 flex items-center gap-2">
                  <MinusCircle className="h-4 w-4" />
                  此次同步未检测到模型变化
                </p>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showClearDialog} onOpenChange={setShowClearDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认清空</AlertDialogTitle>
            <AlertDialogDescription>
              此操作将删除所有模型同步日志，无法撤销。确定要继续吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleClear}>确认清空</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
