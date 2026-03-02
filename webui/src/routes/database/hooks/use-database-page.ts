import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  exportConfig,
  exportDatabase,
  getDatabaseStats,
  importConfig,
  vacuumDatabase,
  type DatabaseStats,
  type ExportType,
  type ImportConfigResponse,
} from "@/lib/api";
import { toast } from "sonner";
import { ALL_EXPORT_TYPES, type ImportMode, type ImportPreviewData } from "../types";
import { buildImportPreview } from "../utils/import-preview";
import { buildImportResultMessage } from "../utils/import-result";

const toErrorMessage = (error: unknown, fallback: string) =>
  error instanceof Error ? error.message : fallback;

export function useDatabasePage() {
  const navigate = useNavigate();
  const previewRequestRef = useRef(0);

  const [stats, setStats] = useState<DatabaseStats | null>(null);
  const [loading, setLoading] = useState(true);

  const [vacuumDialogOpen, setVacuumDialogOpen] = useState(false);
  const [vacuuming, setVacuuming] = useState(false);

  const [exportConfigDialogOpen, setExportConfigDialogOpen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportTypes, setExportTypes] = useState<ExportType[]>([...ALL_EXPORT_TYPES]);

  const [exportDatabaseDialogOpen, setExportDatabaseDialogOpen] = useState(false);
  const [exportingDatabase, setExportingDatabase] = useState(false);

  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importMode, setImportMode] = useState<ImportMode>("merge");
  const [importTypes, setImportTypes] = useState<ExportType[]>([...ALL_EXPORT_TYPES]);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewData, setPreviewData] = useState<ImportPreviewData | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [importFileInputKey, setImportFileInputKey] = useState(0);

  const clearImportFileSelection = useCallback(() => {
    previewRequestRef.current += 1;
    setSelectedFile(null);
    setPreviewData(null);
    setPreviewError(null);
    setPreviewLoading(false);
    setImportFileInputKey((previous) => previous + 1);
  }, []);

  const fetchStats = useCallback(async () => {
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
  }, []);

  useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  const usageRate =
    stats?.page_count && stats.page_count > 0
      ? (((stats.page_count - stats.free_pages) / stats.page_count) * 100).toFixed(1)
      : "0";

  const goBackHome = () => {
    navigate("/");
  };

  const refreshStats = () => {
    void fetchStats();
  };

  const toggleTypeSelection = (
    updater: React.Dispatch<React.SetStateAction<ExportType[]>>,
    type: ExportType
  ) => {
    updater((previous) =>
      previous.includes(type) ? previous.filter((item) => item !== type) : [...previous, type]
    );
  };

  const toggleExportType = (type: ExportType) => {
    toggleTypeSelection(setExportTypes, type);
  };

  const toggleImportType = (type: ExportType) => {
    toggleTypeSelection(setImportTypes, type);
  };

  const handleVacuum = async () => {
    setVacuumDialogOpen(false);
    setVacuuming(true);
    try {
      await vacuumDatabase();
      toast.success("数据库压缩完成");
      await fetchStats();
    } catch (error) {
      toast.error("数据库压缩失败");
      console.error(error);
    } finally {
      setVacuuming(false);
    }
  };

  const handleExportConfig = async () => {
    if (exportTypes.length === 0) {
      toast.error("请至少选择一种数据类型");
      return;
    }

    setExporting(true);
    try {
      await exportConfig(exportTypes);
      toast.success("配置导出成功");
      setExportConfigDialogOpen(false);
    } catch (error) {
      toast.error(`导出失败: ${toErrorMessage(error, "未知错误")}`);
      console.error(error);
    } finally {
      setExporting(false);
    }
  };

  const handleExportDatabase = async () => {
    setExportDatabaseDialogOpen(false);
    setExportingDatabase(true);
    try {
      await exportDatabase();
      toast.success("数据库导出成功");
    } catch (error) {
      toast.error(`导出失败: ${toErrorMessage(error, "未知错误")}`);
      console.error(error);
    } finally {
      setExportingDatabase(false);
    }
  };

  const handleImportDialogOpenChange = (open: boolean) => {
    setImportDialogOpen(open);
    if (!open) {
      clearImportFileSelection();
    }
  };

  const handleImportFileChange = async (file: File | null) => {
    previewRequestRef.current += 1;
    const currentRequest = previewRequestRef.current;

    setSelectedFile(file);
    setPreviewData(null);
    setPreviewError(null);

    if (!file) {
      setPreviewLoading(false);
      return;
    }

    setPreviewLoading(true);
    try {
      const preview = await buildImportPreview(file);
      if (currentRequest !== previewRequestRef.current) {
        return;
      }
      setPreviewData(preview);
    } catch (error) {
      if (currentRequest !== previewRequestRef.current) {
        return;
      }
      setPreviewError("无法解析文件内容，请确保选择的是有效的JSON配置文件");
      console.error("预览文件失败:", error);
    } finally {
      if (currentRequest === previewRequestRef.current) {
        setPreviewLoading(false);
      }
    }
  };

  const handleImportConfig = async () => {
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
      const result: ImportConfigResponse = await importConfig({
        mode: importMode,
        types: importTypes,
        file: selectedFile,
      });
      toast.success(buildImportResultMessage(result), { duration: 5000 });

      setImportDialogOpen(false);
      clearImportFileSelection();
      await fetchStats();
    } catch (error) {
      toast.error(`导入失败: ${toErrorMessage(error, "未知错误")}`);
      console.error(error);
    } finally {
      setImporting(false);
    }
  };

  return {
    stats,
    loading,
    usageRate,
    vacuumDialogOpen,
    vacuuming,
    exportConfigDialogOpen,
    exporting,
    exportTypes,
    exportDatabaseDialogOpen,
    exportingDatabase,
    importDialogOpen,
    importing,
    importMode,
    importTypes,
    selectedFile,
    previewData,
    previewLoading,
    previewError,
    importFileInputKey,
    goBackHome,
    refreshStats,
    handleVacuum,
    handleExportConfig,
    handleExportDatabase,
    handleImportConfig,
    handleImportDialogOpenChange,
    handleImportFileChange,
    toggleExportType,
    toggleImportType,
    setVacuumDialogOpen,
    setExportConfigDialogOpen,
    setExportDatabaseDialogOpen,
    setImportMode,
  };
}
