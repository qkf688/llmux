import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { getModelSyncLogs, deleteModelSyncLogs, clearModelSyncLogs } from "@/lib/api";
import type { ModelSyncLog } from "@/lib/api";
import { toast } from "sonner";
import Loading from "@/components/loading";

export default function ModelSyncLogsPage() {
  const [logs, setLogs] = useState<ModelSyncLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedLogs, setSelectedLogs] = useState<number[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [detailLog, setDetailLog] = useState<ModelSyncLog | null>(null);
  const [showClearDialog, setShowClearDialog] = useState(false);

  useEffect(() => {
    fetchLogs();
  }, [page]);

  const fetchLogs = async () => {
    try {
      setLoading(true);
      const data = await getModelSyncLogs({ page, page_size: 20 });
      setLogs(data.data);
      setTotalPages(data.pagination.total_pages);
    } catch (err) {
      toast.error("加载日志失败: " + (err as Error).message);
    } finally {
      setLoading(false);
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

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString('zh-CN');
  };

  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      <div className="flex items-center justify-between flex-shrink-0">
        <div>
          <h2 className="text-2xl font-bold">模型同步日志</h2>
          <p className="text-sm text-muted-foreground">查看上游模型同步记录</p>
        </div>
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
                      <div className="flex gap-2">
                        {log.AddedCount > 0 && (
                          <span className="text-green-600">+{log.AddedCount}</span>
                        )}
                        {log.RemovedCount > 0 && (
                          <span className="text-red-600">-{log.RemovedCount}</span>
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>同步详情</DialogTitle>
            <DialogDescription>
              {detailLog?.ProviderName} - {detailLog && formatDate(detailLog.SyncedAt)}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
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
