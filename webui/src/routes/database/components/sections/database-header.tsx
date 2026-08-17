import {
  ArrowLeft,
  Database,
  Download,
  RefreshCw,
  Trash2,
  Upload,
} from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

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
    <PageHeader
      icon={Database}
      title="数据库管理"
      subtitle="查看数据库状态和执行维护操作"
      actions={
        <>
          <Button
            variant="outline"
            onClick={onBack}
            className="gap-0 sm:gap-2 px-2 sm:px-3 text-xs sm:text-sm shrink-0"
            size="sm"
            aria-label="返回"
            title="返回"
          >
            <ArrowLeft className="h-4 w-4" />
            <span className="hidden sm:inline">返回</span>
          </Button>
          <Button
            variant="outline"
            onClick={onRefresh}
            disabled={loading}
            className="gap-0 sm:gap-2 px-2 sm:px-3 text-xs sm:text-sm shrink-0"
            size="sm"
            aria-label="刷新"
            title="刷新"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            <span className="hidden sm:inline">刷新</span>
          </Button>
          <Button
            variant="outline"
            onClick={onOpenExportConfig}
            disabled={exporting || loading}
            className="gap-0 sm:gap-2 px-2 sm:px-3 text-xs sm:text-sm shrink-0"
            size="sm"
            aria-label="导出配置"
            title="导出配置"
          >
            <Download className="h-4 w-4" />
            <span className="hidden sm:inline">导出配置</span>
          </Button>
          <Button
            variant="outline"
            onClick={onOpenExportDatabase}
            disabled={exportingDatabase || loading}
            className="gap-0 sm:gap-2 px-2 sm:px-3 text-xs sm:text-sm shrink-0"
            size="sm"
            aria-label="导出数据库"
            title="导出数据库"
          >
            <Database className="h-4 w-4" />
            <span className="hidden sm:inline">导出数据库</span>
          </Button>
          <Button
            variant="outline"
            onClick={onOpenImport}
            disabled={importing || loading}
            className="gap-0 sm:gap-2 px-2 sm:px-3 text-xs sm:text-sm shrink-0"
            size="sm"
            aria-label="导入"
            title="导入"
          >
            <Upload className="h-4 w-4" />
            <span className="hidden sm:inline">导入</span>
          </Button>
          <Button
            variant="destructive"
            onClick={onOpenVacuum}
            disabled={vacuuming || loading}
            className="gap-0 sm:gap-2 px-2 sm:px-3 text-xs sm:text-sm shrink-0"
            size="sm"
            aria-label="压缩"
            title="压缩"
          >
            <Trash2 className={`h-4 w-4 ${vacuuming ? "animate-pulse" : ""}`} />
            <span className="hidden sm:inline">压缩</span>
          </Button>
        </>
      }
    />
  );
}
