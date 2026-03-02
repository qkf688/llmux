import { Button } from "@/components/ui/button";
import {
  ArrowLeft,
  Database,
  Download,
  RefreshCw,
  Trash2,
  Upload,
} from "lucide-react";

type DatabaseHeaderProps = {
  loading: boolean;
  vacuuming: boolean;
  exporting: boolean;
  exportingDatabase: boolean;
  importing: boolean;
  onBack: () => void;
  onRefresh: () => void;
  onOpenExportConfig: () => void;
  onOpenExportDatabase: () => void;
  onOpenImport: () => void;
  onOpenVacuum: () => void;
};

export function DatabaseHeader({
  loading,
  vacuuming,
  exporting,
  exportingDatabase,
  importing,
  onBack,
  onRefresh,
  onOpenExportConfig,
  onOpenExportDatabase,
  onOpenImport,
  onOpenVacuum,
}: DatabaseHeaderProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div className="flex items-center gap-3">
        <Database className="h-8 w-8 text-primary flex-shrink-0" />
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold">数据库管理</h1>
          <p className="text-sm sm:text-base text-muted-foreground">查看数据库状态和执行维护操作</p>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={onBack} className="gap-2 text-xs sm:text-sm" size="sm">
          <ArrowLeft className="h-3 w-3 sm:h-4 sm:w-4" />
          返回
        </Button>
        <Button
          variant="outline"
          onClick={onRefresh}
          disabled={loading}
          className="gap-2 text-xs sm:text-sm"
          size="sm"
        >
          <RefreshCw className={`h-3 w-3 sm:h-4 sm:w-4 ${loading ? "animate-spin" : ""}`} />
          刷新
        </Button>
        <Button
          variant="outline"
          onClick={onOpenExportConfig}
          disabled={exporting || loading}
          className="gap-2 text-xs sm:text-sm"
          size="sm"
        >
          <Download className="h-3 w-3 sm:h-4 sm:w-4" />
          导出配置
        </Button>
        <Button
          variant="outline"
          onClick={onOpenExportDatabase}
          disabled={exportingDatabase || loading}
          className="gap-2 text-xs sm:text-sm"
          size="sm"
        >
          <Database className="h-3 w-3 sm:h-4 sm:w-4" />
          导出数据库
        </Button>
        <Button
          variant="outline"
          onClick={onOpenImport}
          disabled={importing || loading}
          className="gap-2 text-xs sm:text-sm"
          size="sm"
        >
          <Upload className="h-3 w-3 sm:h-4 sm:w-4" />
          导入
        </Button>
        <Button
          variant="destructive"
          onClick={onOpenVacuum}
          disabled={vacuuming || loading}
          className="gap-2 text-xs sm:text-sm"
          size="sm"
        >
          <Trash2 className={`h-3 w-3 sm:h-4 sm:w-4 ${vacuuming ? "animate-pulse" : ""}`} />
          压缩
        </Button>
      </div>
    </div>
  );
}
