import { useState, useEffect, useRef, useCallback } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { LoadingState } from "@/components/ui/loading-state";
import { StatCard, StatGrid } from "@/components/stat-card";
import { getBatchHealthCheckStatus, type BatchHealthCheckStatus } from "@/lib/api";
import { CheckCircle2, XCircle } from "lucide-react";
import { useNavigate } from "react-router-dom";

interface HealthCheckResultDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  batchId: string | null;
}

// 格式化响应时间
const formatTime = (milliseconds: number): string => {
  if (milliseconds < 1) return `${(milliseconds * 1000).toFixed(2)} μs`;
  if (milliseconds < 1000) return `${milliseconds.toFixed(2)} ms`;
  return `${(milliseconds / 1000).toFixed(2)} s`;
};

export function HealthCheckResultDialog({
  open,
  onOpenChange,
  batchId,
}: HealthCheckResultDialogProps) {
  const [status, setStatus] = useState<BatchHealthCheckStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const navigate = useNavigate();

  const fetchStatus = useCallback(async () => {
    if (!batchId) return;

    try {
      const data = await getBatchHealthCheckStatus(batchId);
      setStatus(data);
      setLoading(false);

      // 如果完成，停止轮询
      if (data.completed && intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    } catch (error) {
      console.error("Failed to fetch batch status:", error);
      setLoading(false);
    }
  }, [batchId]);

  useEffect(() => {
    if (!open || !batchId) {
      // 清理定时器
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
      setStatus(null);
      setLoading(true);
      return;
    }

    // 立即获取一次状态
    fetchStatus();

    // 设置轮询
    intervalRef.current = setInterval(() => {
      fetchStatus();
    }, 1500);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [open, batchId, fetchStatus]);

  const handleViewLogs = () => {
    navigate("/health-check-logs");
    onOpenChange(false);
  };

  const handleClose = () => {
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>
            {loading || !status ? "正在启动健康检测..." : "健康检测进度"}
          </DialogTitle>
          <DialogDescription>
            {status?.completed ? (
              "检测已完成"
            ) : (
              <>
                <span>正在对所有模型提供商执行健康检测</span>
                <span className="text-xs text-muted-foreground block mt-1">
                  检测在后台运行，关闭此对话框不会中断检测任务
                </span>
              </>
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-4">
          {loading || !status ? (
            <LoadingState text="" className="py-8" spinnerClassName="w-8 h-8" />
          ) : (
            <>
              {/* 统计概览 */}
              <StatGrid>
                <StatCard label="总计" value={status.total_count} />
                <StatCard label="成功" value={status.success} tone="success" />
                <StatCard label="失败" value={status.failed} tone="danger" />
                <StatCard label="待检测" value={status.pending} tone="info" />
              </StatGrid>

              {/* 进度条 */}
              <div className="space-y-2">
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">进度</span>
                  <span className="font-medium">
                    {status.total_count - status.pending} / {status.total_count}
                  </span>
                </div>
                <div className="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full bg-primary transition-all duration-300"
                    style={{
                      width: `${
                        ((status.total_count - status.pending) / status.total_count) * 100
                      }%`,
                    }}
                  />
                </div>
              </div>

              {/* 最近结果列表 */}
              {status.logs.length > 0 && (
                <div className="space-y-2">
                  <h4 className="text-sm font-medium">最近结果</h4>
                  <div className="rounded-md border max-h-64 overflow-y-auto">
                    <table className="w-full text-sm">
                      <thead className="bg-muted/50 sticky top-0">
                        <tr>
                          <th className="text-left px-3 py-2 font-medium">状态</th>
                          <th className="text-left px-3 py-2 font-medium">模型</th>
                          <th className="text-left px-3 py-2 font-medium">提供商</th>
                          <th className="text-right px-3 py-2 font-medium">响应时间</th>
                        </tr>
                      </thead>
                      <tbody>
                        {status.logs.slice(0, 10).map((log) => (
                          <tr
                            key={log.ID}
                            className="border-t hover:bg-muted/30 transition-colors"
                          >
                            <td className="px-3 py-2">
                              {log.status === "success" ? (
                                <span className="flex items-center gap-1 text-success">
                                  <CheckCircle2 className="w-4 h-4" />
                                  成功
                                </span>
                              ) : (
                                <span className="flex items-center gap-1 text-destructive">
                                  <XCircle className="w-4 h-4" />
                                  失败
                                </span>
                              )}
                            </td>
                            <td className="px-3 py-2 font-mono text-xs">
                              {log.model_name}
                            </td>
                            <td className="px-3 py-2">{log.provider_name}</td>
                            <td className="px-3 py-2 text-right font-mono text-xs">
                              {formatTime(log.response_time)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  {status.logs.length > 10 && (
                    <p className="text-xs text-muted-foreground text-center">
                      仅显示前 10 条结果
                    </p>
                  )}
                </div>
              )}

              {/* 检测中状态 */}
              {!status.completed && (
                <LoadingState text="检测进行中..." className="py-2 text-sm text-muted-foreground" spinnerClassName="w-4 h-4" />
              )}
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleViewLogs}>
            查看详细日志
          </Button>
          <Button onClick={handleClose}>关闭</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
