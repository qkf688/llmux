import { useState, useEffect, useRef } from "react";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Database, HardDrive, Table as TableIcon, RefreshCw, Trash2, ArrowLeft, Download, Upload } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { getDatabaseStats, vacuumDatabase, exportConfig, importConfig, type DatabaseStats, type ExportType } from "@/lib/api";

export default function DatabasePage() {
  const navigate = useNavigate();
  const [stats, setStats] = useState<DatabaseStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [vacuuming, setVacuuming] = useState(false);
  const [showVacuumDialog, setShowVacuumDialog] = useState(false);
  
  // 导出相关状态
  const [showExportDialog, setShowExportDialog] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportTypes, setExportTypes] = useState<ExportType[]>(['providers', 'models', 'associations', 'templates', 'settings']);
  
  // 导入相关状态
  const [showImportDialog, setShowImportDialog] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importMode, setImportMode] = useState<'merge' | 'replace'>('merge');
  const [importTypes, setImportTypes] = useState<ExportType[]>(['providers', 'models', 'associations', 'templates', 'settings']);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

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

  const handleExport = async () => {
    if (exportTypes.length === 0) {
      toast.error("请至少选择一种数据类型");
      return;
    }
    
    setExporting(true);
    try {
      await exportConfig(exportTypes);
      toast.success("配置导出成功");
      setShowExportDialog(false);
    } catch (error) {
      toast.error(`导出失败: ${error instanceof Error ? error.message : '未知错误'}`);
      console.error(error);
    } finally {
      setExporting(false);
    }
  };

  const handleImport = async () => {
    if (!selectedFile) {
      toast.error("请选择要导入的文件");
      return;
    }
    
    if (importTypes.length === 0) {
      toast.error("请至少选择一种数据类型");
      return;
    }
    
    setImporting(true);
    try {
      const result = await importConfig({
        mode: importMode,
        types: importTypes,
        file: selectedFile,
      });
      
      // 构建结果消息
      const messages: string[] = [];
      if (result.providers.imported > 0) messages.push(`提供商: ${result.providers.imported} 条`);
      if (result.models.imported > 0) messages.push(`模型: ${result.models.imported} 条`);
      if (result.associations.imported > 0) messages.push(`关联: ${result.associations.imported} 条`);
      if (result.templates.imported > 0) messages.push(`模板: ${result.templates.imported} 条`);
      if (result.settings.imported > 0) messages.push(`设置: ${result.settings.imported} 条`);
      
      const skippedCount = result.providers.skipped + result.models.skipped +
                          result.associations.skipped + result.templates.skipped + result.settings.skipped;
      
      toast.success(
        `导入成功！已导入: ${messages.join(', ')}${skippedCount > 0 ? `，跳过: ${skippedCount} 条` : ''}`,
        { duration: 5000 }
      );
      
      setShowImportDialog(false);
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
      fetchStats();
    } catch (error) {
      toast.error(`导入失败: ${error instanceof Error ? error.message : '未知错误'}`);
      console.error(error);
    } finally {
      setImporting(false);
    }
  };

  const toggleExportType = (type: ExportType) => {
    setExportTypes(prev =>
      prev.includes(type) ? prev.filter(t => t !== type) : [...prev, type]
    );
  };

  const toggleImportType = (type: ExportType) => {
    setImportTypes(prev =>
      prev.includes(type) ? prev.filter(t => t !== type) : [...prev, type]
    );
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
            variant="outline"
            onClick={() => setShowExportDialog(true)}
            disabled={exporting || loading}
            className="gap-2"
          >
            <Download className="h-4 w-4" />
            导出配置
          </Button>
          <Button
            variant="outline"
            onClick={() => setShowImportDialog(true)}
            disabled={importing || loading}
            className="gap-2"
          >
            <Upload className="h-4 w-4" />
            导入配置
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

      {/* 导出配置对话框 */}
      <Dialog open={showExportDialog} onOpenChange={setShowExportDialog}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>导出配置</DialogTitle>
            <DialogDescription>
              选择要导出的数据类型，系统将生成 JSON 格式的配置文件
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-3">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="export-providers"
                  checked={exportTypes.includes('providers')}
                  onCheckedChange={() => toggleExportType('providers')}
                />
                <Label htmlFor="export-providers" className="cursor-pointer">
                  提供商配置
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="export-models"
                  checked={exportTypes.includes('models')}
                  onCheckedChange={() => toggleExportType('models')}
                />
                <Label htmlFor="export-models" className="cursor-pointer">
                  模型配置
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="export-associations"
                  checked={exportTypes.includes('associations')}
                  onCheckedChange={() => toggleExportType('associations')}
                />
                <Label htmlFor="export-associations" className="cursor-pointer">
                  模型关联
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="export-templates"
                  checked={exportTypes.includes('templates')}
                  onCheckedChange={() => toggleExportType('templates')}
                />
                <Label htmlFor="export-templates" className="cursor-pointer">
                  模型模板
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="export-settings"
                  checked={exportTypes.includes('settings')}
                  onCheckedChange={() => toggleExportType('settings')}
                />
                <Label htmlFor="export-settings" className="cursor-pointer">
                  系统设置
                </Label>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setShowExportDialog(false)}
              disabled={exporting}
            >
              取消
            </Button>
            <Button
              onClick={handleExport}
              disabled={exporting || exportTypes.length === 0}
              className="gap-2"
            >
              <Download className="h-4 w-4" />
              {exporting ? "导出中..." : "导出"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 导入配置对话框 */}
      <Dialog open={showImportDialog} onOpenChange={setShowImportDialog}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>导入配置</DialogTitle>
            <DialogDescription>
              选择配置文件和导入模式，系统将根据您的选择导入数据
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            {/* 文件选择 */}
            <div className="space-y-2">
              <Label>选择配置文件</Label>
              <input
                ref={fileInputRef}
                type="file"
                accept=".json"
                onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              />
              {selectedFile && (
                <p className="text-sm text-muted-foreground">
                  已选择: {selectedFile.name}
                </p>
              )}
            </div>

            {/* 导入模式 */}
            <div className="space-y-2">
              <Label>导入模式</Label>
              <RadioGroup value={importMode} onValueChange={(value) => setImportMode(value as 'merge' | 'replace')}>
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="merge" id="mode-merge" />
                  <Label htmlFor="mode-merge" className="cursor-pointer font-normal">
                    合并模式 - 保留现有数据，仅添加新数据
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="replace" id="mode-replace" />
                  <Label htmlFor="mode-replace" className="cursor-pointer font-normal text-destructive">
                    覆盖模式 - 清空现有数据，完全替换（危险操作）
                  </Label>
                </div>
              </RadioGroup>
            </div>

            {/* 数据类型选择 */}
            <div className="space-y-2">
              <Label>选择要导入的数据类型</Label>
              <div className="space-y-3">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="import-providers"
                    checked={importTypes.includes('providers')}
                    onCheckedChange={() => toggleImportType('providers')}
                  />
                  <Label htmlFor="import-providers" className="cursor-pointer">
                    提供商配置
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="import-models"
                    checked={importTypes.includes('models')}
                    onCheckedChange={() => toggleImportType('models')}
                  />
                  <Label htmlFor="import-models" className="cursor-pointer">
                    模型配置
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="import-associations"
                    checked={importTypes.includes('associations')}
                    onCheckedChange={() => toggleImportType('associations')}
                  />
                  <Label htmlFor="import-associations" className="cursor-pointer">
                    模型关联
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="import-templates"
                    checked={importTypes.includes('templates')}
                    onCheckedChange={() => toggleImportType('templates')}
                  />
                  <Label htmlFor="import-templates" className="cursor-pointer">
                    模型模板
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="import-settings"
                    checked={importTypes.includes('settings')}
                    onCheckedChange={() => toggleImportType('settings')}
                  />
                  <Label htmlFor="import-settings" className="cursor-pointer">
                    系统设置
                  </Label>
                </div>
              </div>
            </div>

            {/* 警告提示 */}
            {importMode === 'replace' && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                ⚠️ 警告：覆盖模式将删除所选类型的所有现有数据！请确保已备份重要数据。
              </div>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setShowImportDialog(false);
                setSelectedFile(null);
                if (fileInputRef.current) {
                  fileInputRef.current.value = '';
                }
              }}
              disabled={importing}
            >
              取消
            </Button>
            <Button
              onClick={handleImport}
              disabled={importing || !selectedFile || importTypes.length === 0}
              className="gap-2"
              variant={importMode === 'replace' ? 'destructive' : 'default'}
            >
              <Upload className="h-4 w-4" />
              {importing ? "导入中..." : "导入"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
