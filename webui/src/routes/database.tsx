import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Database, HardDrive, Table as TableIcon, RefreshCw, Trash2, ArrowLeft } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { getDatabaseStats, vacuumDatabase, type DatabaseStats } from "@/lib/api";

export default function DatabasePage() {
  const navigate = useNavigate();
  const [stats, setStats] = useState<DatabaseStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [vacuuming, setVacuuming] = useState(false);
  const [showVacuumDialog, setShowVacuumDialog] = useState(false);

  const fetchStats = async () => {
    setLoading(true);
    try {
      const data = await getDatabaseStats();
      setStats(data);
    } catch (error) {
      toast.error("获取数据库统计信息失败");
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const handleVacuum = async () => {
    setShowVacuumDialog(false);
    setVacuuming(true);
    try {
      await vacuumDatabase();
      toast.success("数据库压缩完成");
      fetchStats();
    } catch (error) {
      toast.error("数据库压缩失败");
      console.error(error);
    } finally {
      setVacuuming(false);
    }
  };

  useEffect(() => {
    fetchStats();
  }, []);

  const usageRate = stats?.page_count && stats.page_count > 0
    ? ((stats.page_count - stats.free_pages) / stats.page_count * 100).toFixed(1)
    : "0";

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Database className="h-8 w-8 text-primary" />
          <div>
            <h1 className="text-3xl font-bold">数据库管理</h1>
            <p className="text-muted-foreground">查看数据库状态和执行维护操作</p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            onClick={() => navigate("/")}
            className="gap-2"
          >
            <ArrowLeft className="h-4 w-4" />
            返回首页
          </Button>
          <Button
            variant="outline"
            onClick={fetchStats}
            disabled={loading}
            className="gap-2"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            刷新
          </Button>
          <Button
            variant="destructive"
            onClick={() => setShowVacuumDialog(true)}
            disabled={vacuuming || loading}
            className="gap-2"
          >
            <Trash2 className={`h-4 w-4 ${vacuuming ? "animate-pulse" : ""}`} />
            {vacuuming ? "压缩中..." : "VACUUM 压缩"}
          </Button>
        </div>
      </div>

      {loading ? (
        <Card>
          <CardContent className="flex items-center justify-center py-12">
            <div className="text-center">
              <RefreshCw className="h-8 w-8 animate-spin mx-auto mb-4 text-muted-foreground" />
              <p className="text-muted-foreground">加载中...</p>
            </div>
          </CardContent>
        </Card>
      ) : stats ? (
        <>
          {/* 统计卡片 */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">文件大小</CardTitle>
                <HardDrive className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stats.file_size_human}</div>
                <p className="text-xs text-muted-foreground">
                  {stats.file_size.toLocaleString()} 字节
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">使用率</CardTitle>
                <Database className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{usageRate}%</div>
                <p className="text-xs text-muted-foreground">
                  {stats.page_count - stats.free_pages} / {stats.page_count} 页
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">空闲空间</CardTitle>
                <RefreshCw className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stats.free_pages} 页</div>
                <p className="text-xs text-muted-foreground">
                  {(stats.free_pages * stats.page_size).toLocaleString()} 字节
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">页面大小</CardTitle>
                <TableIcon className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stats.page_size.toLocaleString()}</div>
                <p className="text-xs text-muted-foreground">字节/页</p>
              </CardContent>
            </Card>
          </div>

          {/* 表统计 */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <TableIcon className="h-5 w-5" />
                表统计
              </CardTitle>
              <CardDescription>
                数据库中各表的记录数量统计
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>表名</TableHead>
                    <TableHead className="text-right">记录数</TableHead>
                    <TableHead className="text-right">占用空间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stats.table_stats.map((table) => (
                    <TableRow key={table.name}>
                      <TableCell className="font-medium">{table.name}</TableCell>
                      <TableCell className="text-right">
                        {table.count.toLocaleString()}
                      </TableCell>
                      <TableCell className="text-right">
                        {table.estimated_size_human}
                      </TableCell>
                    </TableRow>
                  ))}
                  {stats.table_stats.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={3} className="text-center text-muted-foreground">
                        暂无数据
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          {/* 数据库信息 */}
          <Card>
            <CardHeader>
              <CardTitle>数据库信息</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">数据库路径:</span>
                <span className="font-mono">{stats.db_path}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">数据库版本:</span>
                <span>{stats.sqlite_version}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">编码:</span>
                <span>{stats.encoding}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">最后更新:</span>
                <span>{new Date(stats.last_modified).toLocaleString("zh-CN")}</span>
              </div>
            </CardContent>
          </Card>
        </>
      ) : (
        <Card>
          <CardContent className="flex items-center justify-center py-12">
            <p className="text-muted-foreground">无法加载数据库信息</p>
          </CardContent>
        </Card>
      )}

      {/* VACUUM 确认对话框 */}
      <AlertDialog open={showVacuumDialog} onOpenChange={setShowVacuumDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认执行 VACUUM 操作</AlertDialogTitle>
            <AlertDialogDescription>
              这将压缩数据库并回收空间，可能需要一些时间。在操作期间，数据库将被锁定，无法进行其他操作。确定要继续吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleVacuum}>确认压缩</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
