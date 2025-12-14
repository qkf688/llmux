import { useState, useEffect, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Checkbox } from "@/components/ui/checkbox";
import Loading from "@/components/loading";
import { getLogs, getProviders, getModels, getUserAgents, deleteLog, batchDeleteLogs, type ChatLog, type Provider, type Model, getProviderTemplates, clearAllLogs } from "@/lib/api";
import { ChevronLeft, ChevronRight, RefreshCw, Trash2, Download } from "lucide-react";
import { toast } from "sonner";

// 格式化时间显示
const formatTime = (nanoseconds: number): string => {
  if (nanoseconds < 1000) return `${nanoseconds.toFixed(2)} ns`;
  if (nanoseconds < 1000000) return `${(nanoseconds / 1000).toFixed(2)} μs`;
  if (nanoseconds < 1000000000) return `${(nanoseconds / 1000000).toFixed(2)} ms`;
  return `${(nanoseconds / 1000000000).toFixed(2)} s`;
};

type DetailCardProps = {
  label: string;
  value: ReactNode;
  mono?: boolean;
  maxLines?: number;
};

const DetailCard = ({ label, value, mono = false, maxLines }: DetailCardProps) => (
  <div className="rounded-md border bg-muted/20 p-3 space-y-1">
    <p className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</p>
    <div className={`text-sm break-words ${mono ? 'font-mono text-xs' : ''} ${maxLines ? `max-h-[${maxLines * 1.2}rem] overflow-hidden` : ''}`}>
      {value ?? '-'}
    </div>
  </div>
);

const formatDurationValue = (value?: number) => (typeof value === "number" ? formatTime(value) : "-");
const formatTokenValue = (value?: number) => (typeof value === "number" ? value.toLocaleString() : "-");
const formatTpsValue = (value?: number) => (typeof value === "number" ? value.toFixed(2) : "-");

export default function LogsPage() {
  const [logs, setLogs] = useState<ChatLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [userAgents, setUserAgents] = useState<string[]>([]);
  // 筛选条件
  const [providerNameFilter, setProviderNameFilter] = useState<string>("all");
  const [modelFilter, setModelFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [styleFilter, setStyleFilter] = useState<string>("all");
  const [userAgentFilter, setUserAgentFilter] = useState<string>("all");
  const [availableStyles, setAvailableStyles] = useState<string[]>([]);
  const navigate = useNavigate();
  // 详情弹窗
  const [selectedLog, setSelectedLog] = useState<ChatLog | null>(null);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  // 选择状态
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [isDeleting, setIsDeleting] = useState(false);
  // 删除确认对话框状态
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [batchDeleteDialogOpen, setBatchDeleteDialogOpen] = useState(false);
  const [logToDelete, setLogToDelete] = useState<number | null>(null);
  const [clearAllDialogOpen, setClearAllDialogOpen] = useState(false);
  const [isClearingAll, setIsClearingAll] = useState(false);
  // 获取数据
  const fetchProviders = async () => {
    try {
      const providerList = await getProviders();
      setProviders(providerList);
      const templates = await getProviderTemplates();
      const styleTypes = templates.map(template => template.type);
      setAvailableStyles(styleTypes);
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
  const fetchUserAgents = async () => {
    try {
      const userAgentList = await getUserAgents();
      setUserAgents(userAgentList);
    } catch (error) {
      console.error("Error fetching user agents:", error);
    }
  };
  const fetchLogs = async () => {
    setLoading(true);
    try {
      const result = await getLogs(page, pageSize, {
        providerName: providerNameFilter === "all" ? undefined : providerNameFilter,
        name: modelFilter === "all" ? undefined : modelFilter,
        status: statusFilter === "all" ? undefined : statusFilter,
        style: styleFilter === "all" ? undefined : styleFilter,
        userAgent: userAgentFilter === "all" ? undefined : userAgentFilter
      });
      setLogs(result.data);
      setTotal(result.total);
      setPages(result.pages);
    } catch (error) {
      console.error("Error fetching logs:", error);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    fetchProviders();
    fetchModels();
    fetchUserAgents();
    fetchLogs();
  }, [page, pageSize, providerNameFilter, modelFilter, statusFilter, styleFilter, userAgentFilter]);
  const handleFilterChange = () => {
    setPage(1);
  };
  useEffect(() => {
    handleFilterChange();
  }, [providerNameFilter, modelFilter, statusFilter, styleFilter, userAgentFilter]);
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
  const openDetailDialog = (log: ChatLog) => {
    setSelectedLog(log);
    setIsDialogOpen(true);
  };
  const canViewChatIO = (log: ChatLog) => log.Status === 'success' && log.ChatIO;
  const handleViewChatIO = (log: ChatLog) => {
    if (!canViewChatIO(log)) return;
    navigate(`/logs/${log.ID}/chat-io`);
  };

  // 导出请求响应内容
  const handleExportRequestResponse = (log: ChatLog) => {
    try {
      const exportData = {
        log_id: log.ID,
        created_at: log.CreatedAt,
        model_name: log.Name,
        provider_name: log.ProviderName,
        provider_model: log.ProviderModel,
        status: log.Status,
        request: {
          headers: log.RequestHeaders || null,
          body: log.RequestBody || null,
        },
        response: {
          headers: log.ResponseHeaders || null,
          body: log.ResponseBody || null,
          raw_body: log.RawResponseBody || null,
        },
      };

      const jsonString = JSON.stringify(exportData, null, 2);
      const blob = new Blob([jsonString], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `log-${log.ID}-request-response-${new Date().getTime()}.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      toast.success("导出成功");
    } catch (error) {
      toast.error("导出失败: " + (error as Error).message);
    }
  };

  // 选择相关函数
  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(logs.map(log => log.ID)));
    } else {
      setSelectedIds(new Set());
    }
  };

  const handleSelectOne = (id: number, checked: boolean) => {
    const newSelected = new Set(selectedIds);
    if (checked) {
      newSelected.add(id);
    } else {
      newSelected.delete(id);
    }
    setSelectedIds(newSelected);
  };

  const isAllSelected = logs.length > 0 && selectedIds.size === logs.length;
  const isSomeSelected = selectedIds.size > 0 && selectedIds.size < logs.length;

  // 打开单条删除确认对话框
  const openDeleteDialog = (id: number) => {
    setLogToDelete(id);
    setDeleteDialogOpen(true);
  };

  // 确认删除单条日志
  const confirmDeleteLog = async () => {
    if (logToDelete === null) return;
    try {
      setIsDeleting(true);
      await deleteLog(logToDelete);
      toast.success("日志已删除");
      fetchLogs();
      setSelectedIds(prev => {
        const newSet = new Set(prev);
        newSet.delete(logToDelete);
        return newSet;
      });
    } catch (error) {
      toast.error("删除失败: " + (error as Error).message);
    } finally {
      setIsDeleting(false);
      setDeleteDialogOpen(false);
      setLogToDelete(null);
    }
  };

  // 打开批量删除确认对话框
  const openBatchDeleteDialog = () => {
    if (selectedIds.size === 0) return;
    setBatchDeleteDialogOpen(true);
  };

  // 确认批量删除日志
  const confirmBatchDelete = async () => {
    try {
      setIsDeleting(true);
      const result = await batchDeleteLogs(Array.from(selectedIds));
      toast.success(`已删除 ${result.deleted} 条日志`);
      setSelectedIds(new Set());
      fetchLogs();
    } catch (error) {
      toast.error("批量删除失败: " + (error as Error).message);
    } finally {
      setIsDeleting(false);
      setBatchDeleteDialogOpen(false);
    }
  };

  const handleClearAllLogs = async () => {
    try {
      setIsClearingAll(true);
      const result = await clearAllLogs();
      toast.success(`已清空 ${result.deleted} 条日志`);
      setSelectedIds(new Set());
      fetchLogs();
    } catch (error) {
      toast.error("清空日志失败: " + (error as Error).message);
    } finally {
      setIsClearingAll(false);
      setClearAllDialogOpen(false);
    }
  };

  // 页面切换时清空选择
  useEffect(() => {
    setSelectedIds(new Set());
  }, [page, pageSize, providerNameFilter, modelFilter, statusFilter, styleFilter, userAgentFilter]);
  // 布局开始
  return (
    <div className="h-full min-h-0 flex flex-col gap-4 p-1">
      {/* 顶部标题和刷新 */}
      <div className="flex flex-col gap-2 flex-shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight">请求日志</h2>
          </div>
          <div className="flex gap-2 ml-auto">
            <AlertDialog open={clearAllDialogOpen} onOpenChange={setClearAllDialogOpen}>
              <AlertDialogTrigger asChild>
                <Button
                  variant="destructive"
                  size="sm"
                  className="shrink-0"
                  disabled={isClearingAll}
                >
                  <Trash2 className="size-4 mr-1" />
                  {isClearingAll ? "清空中..." : "清空所有日志"}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>确认清空日志</AlertDialogTitle>
                  <AlertDialogDescription>
                    确定要清空所有请求日志和对话记录吗？此操作不可恢复。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>取消</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={handleClearAllLogs}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    确认清空
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
            {selectedIds.size > 0 && (
              <Button
                onClick={openBatchDeleteDialog}
                variant="destructive"
                size="sm"
                disabled={isDeleting}
                className="shrink-0"
              >
                <Trash2 className="size-4 mr-1" />
                删除 ({selectedIds.size})
              </Button>
            )}
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
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5 lg:gap-4">
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
          <div className="flex flex-col gap-1 text-xs lg:min-w-0">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">类型</Label>
            <Select value={styleFilter} onValueChange={setStyleFilter}>
              <SelectTrigger className="h-8 text-xs w-full px-2">
                <SelectValue placeholder="类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                {availableStyles.map((s) => <SelectItem key={s} value={s}>{s}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1 text-xs col-span-2 sm:col-span-1 lg:col-span-1 lg:min-w-0">
            <Label className="text-[11px] text-muted-foreground uppercase tracking-wide">用户代理</Label>
            <Select value={userAgentFilter} onValueChange={setUserAgentFilter}>
              <SelectTrigger className="h-8 text-xs w-full px-2">
                <SelectValue placeholder="User Agent" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                {userAgents.map((ua) => (
                  <SelectItem key={ua} value={ua}>
                    <span className="truncate max-w-[140px] block">{ua.length > 20 ? ua.substring(0, 20) + '...' : ua}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>
      {/* 列表区域 */}
      <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm">
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading message="加载日志数据" />
          </div>
        ) : logs.length === 0 ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            暂无请求日志
          </div>
        ) : (
          <div className="h-full flex flex-col">
            <div className="flex-1 overflow-y-auto">
              <div className="hidden sm:block w-full">
                <Table className="min-w-[1150px]">
                  <TableHeader className="z-10 sticky top-0 bg-secondary/90 backdrop-blur text-secondary-foreground">
                    <TableRow className="hover:bg-secondary/90">
                      <TableHead className="w-[40px]">
                        <Checkbox
                          checked={isAllSelected}
                          ref={(el) => {
                            if (el) {
                              (el as unknown as HTMLInputElement).indeterminate = isSomeSelected;
                            }
                          }}
                          onCheckedChange={handleSelectAll}
                          aria-label="全选"
                        />
                      </TableHead>
                      <TableHead>时间</TableHead>
                      <TableHead>模型名称</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>Tokens</TableHead>
                      <TableHead>耗时</TableHead>
                      <TableHead>提供商模型</TableHead>
                      <TableHead>类型</TableHead>
                      <TableHead>提供商</TableHead>
                      <TableHead>UA</TableHead>
                      <TableHead className="w-[180px]">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {logs.map((log) => (
                      <TableRow key={log.ID} className={selectedIds.has(log.ID) ? "bg-muted/50" : ""}>
                        <TableCell>
                          <Checkbox
                            checked={selectedIds.has(log.ID)}
                            onCheckedChange={(checked) => handleSelectOne(log.ID, !!checked)}
                            aria-label={`选择日志 ${log.ID}`}
                          />
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                          {new Date(log.CreatedAt).toLocaleString()}
                        </TableCell>
                        <TableCell className="font-medium">{log.Name}</TableCell>
                        <TableCell>
                          <span className={`inline-flex items-center px-2 py-1 ${log.Status === 'success' ? 'text-green-500' : 'text-red-500 '
                            }`}>
                            {log.Status}
                          </span>
                        </TableCell>
                        <TableCell>{log.total_tokens}</TableCell>
                        <TableCell>{formatTime(log.ChunkTime)}</TableCell>
                        <TableCell className="max-w-[120px] truncate text-xs" title={log.ProviderModel}>{log.ProviderModel}</TableCell>
                        <TableCell className="text-xs">{log.Style}</TableCell>
                        <TableCell className="text-xs">{log.ProviderName}</TableCell>
                        <TableCell className="max-w-[100px] truncate text-xs" title={log.UserAgent}>
                          {log.UserAgent || '-'}
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button variant="ghost" size="sm" className="h-8 px-2" onClick={() => openDetailDialog(log)}>
                              详情
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 px-2"
                              onClick={() => handleViewChatIO(log)}
                              disabled={!canViewChatIO(log)}
                            >
                              会话
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 px-2 text-destructive hover:text-destructive"
                              onClick={() => openDeleteDialog(log.ID)}
                              disabled={isDeleting}
                            >
                              <Trash2 className="size-4" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div className="sm:hidden px-2 py-3 divide-y divide-border">
                {logs.map((log) => (
                  <div key={log.ID} className={`py-3 space-y-2 my-1 px-1 ${selectedIds.has(log.ID) ? 'bg-muted/50 rounded' : ''}`}>
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-start gap-2 min-w-0 flex-1">
                        <Checkbox
                          checked={selectedIds.has(log.ID)}
                          onCheckedChange={(checked) => handleSelectOne(log.ID, !!checked)}
                          className="mt-1"
                        />
                        <div className="min-w-0 flex-1">
                          <h3 className="font-semibold text-sm truncate">{log.Name}</h3>
                          <p className="text-[11px] text-muted-foreground">{new Date(log.CreatedAt).toLocaleString()}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span
                          className={`text-[11px] font-medium px-2 py-0.5 rounded-full ${log.Status === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                            }`}
                        >
                          {log.Status}
                        </span>
                        <div className="flex gap-1">
                          <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => openDetailDialog(log)}>
                            详情
                          </Button>
                          <Button
                            size="sm"
                            className="h-7 px-2 text-xs"
                            onClick={() => handleViewChatIO(log)}
                            disabled={!canViewChatIO(log)}
                          >
                            会话
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 px-2 text-xs text-destructive"
                            onClick={() => openDeleteDialog(log.ID)}
                            disabled={isDeleting}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-3 text-xs ml-6">
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">Tokens</p>
                        <p className="font-medium">{log.total_tokens}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">耗时</p>
                        <p className="font-medium">{formatTime(log.ChunkTime)}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">提供商</p>
                        <p className="truncate">{log.ProviderName}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">类型</p>
                        <p>{log.Style || '-'}</p>
                      </div>
                    </div>
                    {/* 显示请求响应内容大小提示 */}
                    <div className="grid grid-cols-2 gap-3 text-xs ml-6">
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">请求头</p>
                        <p className="font-medium">{log.RequestHeaders ? `${log.RequestHeaders.length} 字节` : '-'}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground text-[10px] uppercase tracking-wide">响应头</p>
                        <p className="font-medium">{log.ResponseHeaders ? `${log.ResponseHeaders.length} 字节` : '-'}</p>
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
                <DialogTitle>日志详情: {selectedLog.ID}</DialogTitle>
              </DialogHeader>
            </div>
            <div className="overflow-y-auto p-3 flex-1">
              <div className="space-y-6 text-sm">
                <div className="space-y-3">
                  <div className="space-y-2">
                    <div className="text-sm">
                      <span className="text-muted-foreground">创建时间：</span>
                      <span>{new Date(selectedLog.CreatedAt).toLocaleString()}</span>
                    </div>
                    <div className="text-sm">
                      <span className="text-muted-foreground">状态：</span>
                      <span className={selectedLog.Status === 'success' ? 'text-green-600' : 'text-red-600'}>
                        {selectedLog.Status}
                      </span>
                    </div>
                  </div>
                </div>
                {selectedLog.Error && (
                  <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3">
                    <p className="text-xs text-destructive uppercase tracking-wide mb-1">错误信息</p>
                    <div className="text-destructive whitespace-pre-wrap break-words text-sm">
                      {selectedLog.Error}
                    </div>
                  </div>
                )}
                <div className="space-y-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">基本信息</p>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <DetailCard label="模型名称" value={selectedLog.Name} />
                    <DetailCard label="提供商" value={selectedLog.ProviderName || '-'} />
                    <DetailCard label="提供商模型" value={selectedLog.ProviderModel || '-'} mono />
                    <DetailCard label="类型" value={selectedLog.Style || '-'} />
                    <DetailCard label="用户代理" value={selectedLog.UserAgent || '-'} mono />
                    <DetailCard label="远端 IP" value={selectedLog.RemoteIP || '-'} mono />
                    <DetailCard label="记录 IO" value={selectedLog.ChatIO ? '是' : '否'} />
                    <DetailCard label="重试次数" value={selectedLog.Retry ?? 0} />
                  </div>
                </div>
                {(selectedLog.RequestHeaders || selectedLog.RequestBody || selectedLog.ResponseHeaders || selectedLog.RawResponseBody || selectedLog.ResponseBody) && (
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">请求响应内容</p>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleExportRequestResponse(selectedLog)}
                      >
                        <Download className="size-4 mr-1" />
                        导出请求响应
                      </Button>
                    </div>
                    <div className="space-y-4">
                      {selectedLog.RequestHeaders && (
                        <div className="rounded-md border bg-muted/20 p-3 space-y-1">
                          <p className="text-[11px] text-muted-foreground uppercase tracking-wide">请求头 ({selectedLog.RequestHeaders.length} 字节)</p>
                          <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
                            {selectedLog.RequestHeaders}
                          </pre>
                        </div>
                      )}
                      {selectedLog.RequestBody && (
                        <div className="rounded-md border bg-muted/20 p-3 space-y-1">
                          <p className="text-[11px] text-muted-foreground uppercase tracking-wide">请求体 ({selectedLog.RequestBody.length} 字节)</p>
                          <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
                            {selectedLog.RequestBody}
                          </pre>
                        </div>
                      )}
                      {selectedLog.ResponseHeaders && (
                        <div className="rounded-md border bg-muted/20 p-3 space-y-1">
                          <p className="text-[11px] text-muted-foreground uppercase tracking-wide">响应头 ({selectedLog.ResponseHeaders.length} 字节)</p>
                          <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
                            {selectedLog.ResponseHeaders}
                          </pre>
                        </div>
                      )}
                      {selectedLog.RawResponseBody && (
                        <div className="rounded-md border bg-blue-50 dark:bg-blue-950/20 p-3 space-y-1">
                          <p className="text-[11px] text-blue-700 dark:text-blue-400 uppercase tracking-wide">
                            {selectedLog.ResponseBody ? '原始响应体 - 转换前' : '响应体'} ({selectedLog.RawResponseBody.length} 字节)
                          </p>
                          <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-blue-900 dark:text-blue-200">
                            {selectedLog.RawResponseBody}
                          </pre>
                        </div>
                      )}
                      {selectedLog.ResponseBody && (
                        <div className="rounded-md border bg-green-50 dark:bg-green-950/20 p-3 space-y-1">
                          <p className="text-[11px] text-green-700 dark:text-green-400 uppercase tracking-wide">响应体 - 转换后 ({selectedLog.ResponseBody.length} 字节)</p>
                          <pre className="text-xs font-mono whitespace-pre-wrap break-words max-h-40 overflow-y-auto text-green-900 dark:text-green-200">
                            {selectedLog.ResponseBody}
                          </pre>
                        </div>
                      )}
                    </div>
                  </div>
                )}
                <div className="space-y-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">性能指标</p>
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                    <DetailCard label="代理耗时" value={formatDurationValue(selectedLog.ProxyTime)} />
                    <DetailCard label="首包耗时" value={formatDurationValue(selectedLog.FirstChunkTime)} />
                    <DetailCard label="完成耗时" value={formatDurationValue(selectedLog.ChunkTime)} />
                    <DetailCard label="TPS" value={formatTpsValue(selectedLog.Tps)} />
                  </div>
                </div>
                <div className="space-y-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Token 使用</p>
                  <div className="grid grid-cols-1 sm:grid-cols-4 gap-4">
                    <DetailCard label="输入" value={formatTokenValue(selectedLog.prompt_tokens)} />
                    <DetailCard label="输出" value={formatTokenValue(selectedLog.completion_tokens)} />
                    <DetailCard label="总计" value={formatTokenValue(selectedLog.total_tokens)} />
                    <DetailCard label="缓存" value={formatTokenValue(selectedLog.prompt_tokens_details.cached_tokens)} />
                  </div>
                </div>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      )}

      {/* 单条删除确认对话框 */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除这条日志吗？此操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDeleteLog}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 批量删除确认对话框 */}
      <AlertDialog open={batchDeleteDialogOpen} onOpenChange={setBatchDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认批量删除</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除选中的 {selectedIds.size} 条日志吗？此操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmBatchDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
 
