import { useState, useEffect, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import Loading from "@/components/loading";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { getHealthCheckLogs, getProviders, getModels, clearHealthCheckLogs, type HealthCheckLog, type Provider, type Model } from "@/lib/api";
import { ChevronLeft, ChevronRight, RefreshCw } from "lucide-react";
import { toast } from "sonner";

// 格式化时间显示
const formatTime = (milliseconds: number): string => {
  if (milliseconds < 1) return `${(milliseconds * 1000).toFixed(2)} μs`;
  if (milliseconds < 1000) return `${milliseconds.toFixed(2)} ms`;
  return `${(milliseconds / 1000).toFixed(2)} s`;
};

type DetailCardProps = {
  label: string;
  value: ReactNode;
  mono?: boolean;
};

const DetailCard = ({ label, value, mono = false }: DetailCardProps) => (
  <div className="rounded-md border bg-muted/20 p-3 space-y-1">
    <p className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</p>
    <div className={`text-sm break-words ${mono ? 'font-mono text-xs' : ''}`}>
      {value ?? '-'}
    </div>
  </div>
);

export default function HealthCheckLogsPage() {
  const [logs, setLogs] = useState<HealthCheckLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  // 筛选条件
  const [providerNameFilter, setProviderNameFilter] = useState<string>("all");
  const [modelFilter, setModelFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  // 详情弹窗
  const [selectedLog, setSelectedLog] = useState<HealthCheckLog | null>(null);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [isClearDialogOpen, setIsClearDialogOpen] = useState(false);
  const [clearingLogs, setClearingLogs] = useState(false);

  // 获取数据
  const fetchProviders = async () => {
    try {
      const providerList = await getProviders();
      setProviders(providerList);
    } catch (error) {
      console.error("Error fetching providers:", error);
    }
  };

  const fetchModels = async () => {
    try {
      const modelList = await getModels();
      setModels(modelList);
    } catch (error) {
      console.error("Error fetching models:", error);
    }
  };

  const fetchLogs = async (pageToFetch = page, pageSizeToUse = pageSize) => {
    setLoading(true);
    try {
      const result = await getHealthCheckLogs(pageToFetch, pageSizeToUse, {
        providerName: providerNameFilter === "all" ? undefined : providerNameFilter,
        modelName: modelFilter === "all" ? undefined : modelFilter,
        status: statusFilter === "all" ? undefined : statusFilter,
      });
      setLogs(result.data);
      setTotal(result.total);
      setPages(result.pages);
    } catch (error) {
      console.error("Error fetching health check logs:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProviders();
    fetchModels();
  }, []);

  useEffect(() => {
    fetchLogs();
  }, [page, pageSize, providerNameFilter, modelFilter, statusFilter]);

  const handleFilterChange = () => {
    setPage(1);
  };

  useEffect(() => {
    handleFilterChange();
  }, [providerNameFilter, modelFilter, statusFilter]);

  const handlePageChange = (newPage: number) => {
    if (newPage >= 1 && newPage <= pages) setPage(newPage);
  };

  const handlePageSizeChange = (size: number) => {
    if (size === pageSize) return;
    setPage(1);
    setPageSize(size);
  };

  const handleRefresh = () => {
    fetchLogs();
  };

  const openDetailDialog = (log: HealthCheckLog) => {
    setSelectedLog(log);
    setIsDialogOpen(true);
  };

  const handleClearLogs = async () => {
    try {
      setClearingLogs(true);
      const result = await clearHealthCheckLogs();
      toast.success(`已清空 ${result.deleted} 条健康检测日志`);
      setIsClearDialogOpen(false);
      setPage(1);
      await fetchLogs(1, pageSize);
    } catch (error) {
      toast.error("清空健康检测日志失败: " + (error as Error).message);
    } finally {
      setClearingLogs(false);
    }
  };

  // 布局开始
  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      {/* 顶部标题和刷新 */}
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">健康检测日志</h2>
            <p className="text-sm text-muted-foreground">查看模型提供商的健康检测历史记录</p>
          </div>
          <div className="flex gap-2 ml-auto">
            <AlertDialog open={isClearDialogOpen} onOpenChange={setIsClearDialogOpen}>
              <AlertDialogTrigger asChild>
                <Button
                  variant="destructive"
                  className="shrink-0"
                  disabled={clearingLogs}
                >
                  {clearingLogs ? "清空中..." : "清空检测日志"}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>确认清空检测日志</AlertDialogTitle>
                  <AlertDialogDescription>
                    删除所有健康检测日志，操作不可恢复。确认继续？
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>取消</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={handleClearLogs}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    disabled={clearingLogs}
                  >
                    {clearingLogs ? "清空中..." : "确认清空"}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
            <Button
              onClick={handleRefresh}
              variant="outline"
              size="icon"
              className="shrink-0"
              aria-label="刷新列表"
              title="刷新列表"
            >
              <RefreshCw className="size-4" />
            </Button>
          </div>
        </div>
      </div>

      {/* 筛选区域 */}
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:gap-4">
          <div className="flex flex-col gap-1 text-xs lg:min-w-0">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">模型名称</Label>
            <Select value={modelFilter} onValueChange={setModelFilter}>
              <SelectTrigger className="h-8 text-xs w-full px-2">
                <SelectValue placeholder="选择模型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                {models.map((model) => (
                  <SelectItem key={model.ID} value={model.Name}>{model.Name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1 text-xs lg:min-w-0">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">提供商</Label>
            <Select value={providerNameFilter} onValueChange={setProviderNameFilter}>
              <SelectTrigger className="h-8 text-xs w-full px-2">
                <SelectValue placeholder="选择提供商" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                {providers.map((p) => (
                  <SelectItem key={p.ID} value={p.Name}>{p.Name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1 text-xs lg:min-w-0">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">状态</Label>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="h-8 text-xs w-full px-2">
                <SelectValue placeholder="状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                <SelectItem value="success">成功</SelectItem>
                <SelectItem value="error">错误</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      {/* 列表区域 */}
      <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading message="加载健康检测日志" />
          </div>
        ) : logs.length === 0 ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            暂无健康检测日志
          </div>
        ) : (
          <div className="h-full flex flex-col">
            <div className="flex-1 overflow-y-auto">
              <div className="hidden sm:block w-full">
                <Table className="min-w-[900px]">
                  <TableHeader className="z-10 sticky top-0 bg-secondary/90 backdrop-blur text-secondary-foreground">
                    <TableRow className="hover:bg-secondary/90">
                      <TableHead>检测时间</TableHead>
                      <TableHead>模型名称</TableHead>
                      <TableHead>提供商</TableHead>
                      <TableHead>提供商模型</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>响应时间</TableHead>
                      <TableHead className="w-[100px]">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {logs.map((log) => (
                      <TableRow key={log.ID}>
                        <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                          {new Date(log.checked_at).toLocaleString()}
                        </TableCell>
                        <TableCell className="font-medium">{log.model_name}</TableCell>
                        <TableCell className="text-xs">{log.provider_name}</TableCell>
                        <TableCell className="max-w-[150px] truncate text-xs" title={log.provider_model}>
                          {log.provider_model}
                        </TableCell>
                        <TableCell>
                          <span className={`inline-flex items-center px-2 py-1 ${log.status === 'success' ? 'text-green-500' : 'text-red-500'}`}>
                            {log.status === 'success' ? '成功' : '失败'}
                          </span>
                        </TableCell>
                        <TableCell>{formatTime(log.response_time)}</TableCell>
                        <TableCell>
                          <Button variant="ghost" size="sm" className="h-8 px-2" onClick={() => openDetailDialog(log)}>
                            详情
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* 移动端卡片视图 */}
              <div className="sm:hidden px-2 py-3 divide-y divide-border">
                {logs.map((log) => (
                  <div key={log.ID} className="py-3 space-y-2 my-1 px-1">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <h3 className="font-semibold text-sm truncate">{log.model_name}</h3>
                        <p className="text-[11px] text-muted-foreground">{new Date(log.checked_at).toLocaleString()}</p>
                      </div>
                      <div className="flex items-center gap-2">
                        <span
                          className={`text-[11px] font-medium px-2 py-0.5 rounded-full ${
                            log.status === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                          }`}
                        >
                          {log.status === 'success' ? '成功' : '失败'}
                        </span>
                        <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => openDetailDialog(log)}>
                          详情
                        </Button>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-3 text-xs">
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">提供商</p>
                        <p className="truncate">{log.provider_name}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">响应时间</p>
                        <p className="font-medium">{formatTime(log.response_time)}</p>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* 分页区域 */}
      <div className="flex flex-wrap items-center justify-between gap-3 flex-shrink-0 border-t pt-2">
        <div className="text-sm text-muted-foreground whitespace-nowrap">
          共 {total} 条，第 {page} / {pages} 页
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Select value={String(pageSize)} onValueChange={(value) => handlePageSizeChange(Number(value))}>
              <SelectTrigger className="h-8 text-xs">
                <SelectValue placeholder="条数" />
              </SelectTrigger>
              <SelectContent>
                {[10, 20, 50].map((size) => (
                  <SelectItem key={size} value={String(size)}>
                    {size}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="icon"
              onClick={() => handlePageChange(page - 1)}
              disabled={page === 1}
              aria-label="上一页"
            >
              <ChevronLeft className="size-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              onClick={() => handlePageChange(page + 1)}
              disabled={page === pages}
              aria-label="下一页"
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      </div>

      {/* 详情弹窗 */}
      {selectedLog && (
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogContent className="p-0 w-[92vw] sm:w-auto sm:max-w-2xl max-h-[95vh] flex flex-col">
            <div className="p-4 border-b flex-shrink-0">
              <DialogHeader className="p-0">
                <DialogTitle>检测详情: {selectedLog.ID}</DialogTitle>
              </DialogHeader>
            </div>
            <div className="overflow-y-auto p-3 flex-1">
              <div className="space-y-6 text-sm">
                <div className="space-y-3">
                  <div className="space-y-2">
                    <div className="text-sm">
                      <span className="text-muted-foreground">检测时间：</span>
                      <span>{new Date(selectedLog.checked_at).toLocaleString()}</span>
                    </div>
                    <div className="text-sm">
                      <span className="text-muted-foreground">状态：</span>
                      <span className={selectedLog.status === 'success' ? 'text-green-600' : 'text-red-600'}>
                        {selectedLog.status === 'success' ? '成功' : '失败'}
                      </span>
                    </div>
                  </div>
                </div>

                {selectedLog.error && (
                  <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3">
                    <p className="text-xs text-destructive uppercase tracking-wide mb-1">错误信息</p>
                    <div className="text-destructive whitespace-pre-wrap break-words text-sm">
                      {selectedLog.error}
                    </div>
                  </div>
                )}

                <div className="space-y-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">基本信息</p>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <DetailCard label="模型名称" value={selectedLog.model_name} />
                    <DetailCard label="提供商" value={selectedLog.provider_name || '-'} />
                    <DetailCard label="提供商模型" value={selectedLog.provider_model || '-'} mono />
                    <DetailCard label="响应时间" value={formatTime(selectedLog.response_time)} />
                    <DetailCard label="模型提供商ID" value={selectedLog.model_provider_id} />
                  </div>
                </div>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
