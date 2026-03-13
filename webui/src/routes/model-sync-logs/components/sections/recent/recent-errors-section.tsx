import Loading from "@/components/loading";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { ModelSyncLog, Provider } from "@/lib/api";
import { cn } from "@/lib/utils";
import { CheckSquare2, Trash2 } from "lucide-react";
import { useMemo } from "react";
import { formatSyncDate } from "../../../utils/formatters";

type RecentErrorsSectionProps = {
  loading: boolean;
  logs: ModelSyncLog[];
  providersById: Record<number, Provider | undefined>;
  togglingProviderIds: Set<number>;
  selectedCount: number;
  allSelected: boolean;
  clearing: boolean;
  onToggleModelEndpoint: (providerId: number, enabled: boolean) => void;
  onToggleSelectAll: () => void;
  isProviderSelected: (providerId: number) => boolean;
  onToggleSelectProvider: (providerId: number, checked: boolean) => void;
  onClearSelected: () => void;
  onClearAll: () => void;
  onOpenDetail: (log: ModelSyncLog) => void;
};

type ProviderErrorSummary = {
  providerId: number;
  providerName: string;
  lastLog: ModelSyncLog;
  count: number;
};

function isModelEndpointEnabled(provider?: Provider): boolean {
  return provider?.ModelEndpoint !== false;
}

export function RecentErrorsSection({
  loading,
  logs,
  providersById,
  togglingProviderIds,
  selectedCount,
  allSelected,
  clearing,
  onToggleModelEndpoint,
  onToggleSelectAll,
  isProviderSelected,
  onToggleSelectProvider,
  onClearSelected,
  onClearAll,
  onOpenDetail,
}: RecentErrorsSectionProps) {
  const summaries = useMemo(() => {
    const map = new Map<number, ProviderErrorSummary>();
    for (const log of logs) {
      const existing = map.get(log.ProviderID);
      if (!existing) {
        map.set(log.ProviderID, {
          providerId: log.ProviderID,
          providerName: log.ProviderName,
          lastLog: log,
          count: 1,
        });
        continue;
      }
      existing.count += 1;
    }
    return Array.from(map.values()).sort(
      (a, b) => new Date(b.lastLog.SyncedAt).getTime() - new Date(a.lastLog.SyncedAt).getTime()
    );
  }, [logs]);

  const providerCount = summaries.length;
  const isSomeSelected = selectedCount > 0 && !allSelected;

  return (
    <div className="flex-1 min-h-0 border rounded-md bg-background shadow-sm flex flex-col">
      {loading ? (
        <div className="flex flex-1 items-center justify-center">
          <Loading message="加载最近错误" />
        </div>
      ) : summaries.length === 0 ? (
        <div className="flex flex-1 items-center justify-center text-muted-foreground text-sm">
          暂无最近错误
        </div>
      ) : (
        <>
          <div className="p-3 border-b bg-muted/30 flex items-center justify-end">
            <div className="flex items-center justify-end gap-2 flex-wrap">
              <Button
                variant="outline"
                size="sm"
                className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
                onClick={onToggleSelectAll}
                disabled={providerCount === 0 || clearing}
              >
                <CheckSquare2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 mr-1" />
                <span className="hidden sm:inline">{allSelected && providerCount > 0 ? "取消全选" : "全选"}</span>
                <span className="sm:hidden">{allSelected && providerCount > 0 ? "取消" : "全选"}</span>
              </Button>

              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    variant="destructive"
                    size="sm"
                    className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
                    disabled={selectedCount === 0 || clearing}
                  >
                    <Trash2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 mr-1" />
                    <span className="hidden sm:inline">清除所选 ({selectedCount})</span>
                    <span className="sm:hidden">清除({selectedCount})</span>
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>清除所选错误日志？</AlertDialogTitle>
                    <AlertDialogDescription>
                      将删除所选提供商的全部错误同步日志（Status=error），无法撤销。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={clearing}>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={onClearSelected} disabled={clearing}>
                      {clearing ? "清除中..." : "确认清除"}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>

              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    variant="destructive"
                    size="sm"
                    className="sm:h-9 h-7 sm:px-4 px-2 sm:text-sm text-xs"
                    disabled={logs.length === 0 || clearing}
                  >
                    <Trash2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 mr-1" />
                    <span className="hidden sm:inline">清空错误</span>
                    <span className="sm:hidden">清空</span>
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>清空全部错误日志？</AlertDialogTitle>
                    <AlertDialogDescription>将删除数据库内所有错误同步日志（Status=error），无法撤销。</AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={clearing}>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={onClearAll} disabled={clearing}>
                      {clearing ? "清空中..." : "确认清空"}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </div>
          <div className="flex-1 min-h-0 overflow-auto">
            <div className="hidden sm:block w-full">
              <Table>
                <TableHeader className="sticky top-0 bg-secondary/80">
                  <TableRow>
                    <TableHead className="w-12">
                      <Checkbox
                        checked={isSomeSelected ? "indeterminate" : allSelected}
                        onCheckedChange={() => onToggleSelectAll()}
                        disabled={clearing}
                        aria-label="全选"
                      />
                    </TableHead>
                    <TableHead>提供商</TableHead>
                    <TableHead>最近错误</TableHead>
                    <TableHead>最近时间</TableHead>
                    <TableHead className="text-right">次数</TableHead>
                    <TableHead className="text-right">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {summaries.map((item) => {
                    const provider = providersById[item.providerId];
                    const providerMissing = provider == null;
                    const enabled = providerMissing ? false : isModelEndpointEnabled(provider);
                    const isToggling = togglingProviderIds.has(item.providerId);

                    const selected = isProviderSelected(item.providerId);

                    return (
                      <TableRow key={item.providerId} className={selected ? "bg-muted/40" : ""}>
                        <TableCell>
                          <Checkbox
                            checked={selected}
                            onCheckedChange={(checked) => onToggleSelectProvider(item.providerId, checked === true)}
                            disabled={clearing}
                          />
                        </TableCell>
                        <TableCell className="font-medium">
                          <div className="flex items-center gap-2">
                            <span>{item.providerName}</span>
                            <span
                              className={cn(
                                "text-xs px-2 py-0.5 rounded border",
                                providerMissing
                                  ? "bg-muted border-border text-muted-foreground"
                                  : enabled
                                    ? "bg-emerald-50 dark:bg-emerald-950/20 border-emerald-200 dark:border-emerald-900 text-emerald-700 dark:text-emerald-400"
                                    : "bg-amber-50 dark:bg-amber-950/20 border-amber-200 dark:border-amber-900 text-amber-700 dark:text-amber-400"
                              )}
                            >
                              模型端点: {providerMissing ? "不可用" : enabled ? "开启" : "关闭"}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="max-w-[520px]">
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 px-2 text-xs"
                            onClick={() => onOpenDetail(item.lastLog)}
                          >
                            查看错误
                          </Button>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {formatSyncDate(item.lastLog.SyncedAt)}
                        </TableCell>
                        <TableCell className="text-right font-mono">{item.count}</TableCell>
                        <TableCell className="text-right">
                          <div
                            className="flex justify-end"
                            onClick={(event) => {
                              event.stopPropagation();
                            }}
                          >
                            {providerMissing ? (
                              <Button variant="outline" size="sm" className="h-7 px-2 text-xs" disabled>
                                已删除
                              </Button>
                            ) : enabled ? (
                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button
                                    variant="destructive"
                                    size="sm"
                                    className="h-7 px-2 text-xs"
                                    disabled={isToggling}
                                  >
                                    {isToggling ? "关闭中..." : "关闭模型端点"}
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>关闭提供商模型端点？</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      关闭后将跳过该提供商的自动模型同步（不会再请求上游模型列表）。
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>取消</AlertDialogCancel>
                                    <AlertDialogAction
                                      onClick={() => onToggleModelEndpoint(item.providerId, false)}
                                      disabled={isToggling}
                                    >
                                      确认关闭
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            ) : (
                              <Button
                                variant="outline"
                                size="sm"
                                className="h-7 px-2 text-xs"
                                disabled={isToggling}
                                onClick={() => onToggleModelEndpoint(item.providerId, true)}
                              >
                                {isToggling ? "开启中..." : "重新开启"}
                              </Button>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
            <div className="sm:hidden px-2 py-3 divide-y divide-border">
              {summaries.map((item) => {
                const provider = providersById[item.providerId];
                const providerMissing = provider == null;
                const enabled = providerMissing ? false : isModelEndpointEnabled(provider);
                const isToggling = togglingProviderIds.has(item.providerId);
                const selected = isProviderSelected(item.providerId);

                return (
                  <div
                    key={item.providerId}
                    className={`py-3 space-y-2 my-1 px-1 ${selected ? "bg-muted/50 rounded" : ""}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2 min-w-0 flex-1">
                        <Checkbox
                          checked={selected}
                          onCheckedChange={(checked) => onToggleSelectProvider(item.providerId, checked === true)}
                          disabled={clearing}
                        />
                        <div className="font-medium truncate">{item.providerName}</div>
                      </div>
                      <span
                        className={cn(
                          "text-xs px-2 py-0.5 rounded border",
                          providerMissing
                            ? "bg-muted border-border text-muted-foreground"
                            : enabled
                              ? "bg-emerald-50 dark:bg-emerald-950/20 border-emerald-200 dark:border-emerald-900 text-emerald-700 dark:text-emerald-400"
                              : "bg-amber-50 dark:bg-amber-950/20 border-amber-200 dark:border-amber-900 text-amber-700 dark:text-amber-400"
                        )}
                      >
                        模型端点: {providerMissing ? "不可用" : enabled ? "开启" : "关闭"}
                      </span>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {formatSyncDate(item.lastLog.SyncedAt)} · 次数 <span className="font-mono">{item.count}</span>
                    </div>
                    <div
                      className="flex gap-1.5"
                      onClick={(event) => {
                        event.stopPropagation();
                      }}
                    >
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 px-2 text-[11px]"
                        onClick={() => onOpenDetail(item.lastLog)}
                      >
                        查看
                      </Button>
                      {providerMissing ? (
                        <Button variant="outline" size="sm" className="h-7 px-2 text-[11px]" disabled>
                          已删除
                        </Button>
                      ) : enabled ? (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button
                              variant="destructive"
                              size="sm"
                              className="h-7 px-2 text-[11px]"
                              disabled={isToggling}
                            >
                              {isToggling ? "关闭中" : "关闭"}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>关闭提供商模型端点？</AlertDialogTitle>
                              <AlertDialogDescription>
                                关闭后将跳过该提供商的自动模型同步（不会再请求上游模型列表）。
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>取消</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() => onToggleModelEndpoint(item.providerId, false)}
                                disabled={isToggling}
                              >
                                确认关闭
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 px-2 text-[11px]"
                          disabled={isToggling}
                          onClick={() => onToggleModelEndpoint(item.providerId, true)}
                        >
                          {isToggling ? "开启中" : "开启"}
                        </Button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
