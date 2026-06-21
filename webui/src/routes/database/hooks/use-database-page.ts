import { useCallback, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  exportConfig,
  exportDatabase,
  importConfig,
  vacuumDatabase,
  type ImportConfigResponse,
} from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import { toast } from "sonner";
import { buildImportPreview } from "../utils/import-preview";
import { buildImportResultMessage } from "../utils/import-result";
import {
  selectBumpDatabaseImportFileInputKey,
  selectDatabaseExportConfigDialogOpen,
  selectDatabaseExportDatabaseDialogOpen,
  selectDatabaseExportTypes,
  selectDatabaseExporting,
  selectDatabaseExportingDatabase,
  selectDatabaseImportDialogOpen,
  selectDatabaseImportFileInputKey,
  selectDatabaseImportMode,
  selectDatabaseImportTypes,
  selectDatabaseImporting,
  selectDatabasePreviewData,
  selectDatabasePreviewError,
  selectDatabasePreviewLoading,
  selectDatabaseSelectedFile,
  selectDatabaseVacuumDialogOpen,
  selectDatabaseVacuuming,
  selectResetDatabaseTransient,
  selectSetDatabaseExportConfigDialogOpen,
  selectSetDatabaseExportDatabaseDialogOpen,
  selectSetDatabaseExporting,
  selectSetDatabaseExportingDatabase,
  selectSetDatabaseImportDialogOpen,
  selectSetDatabaseImportMode,
  selectSetDatabaseImporting,
  selectSetDatabasePreviewData,
  selectSetDatabasePreviewError,
  selectSetDatabasePreviewLoading,
  selectSetDatabaseSelectedFile,
  selectSetDatabaseVacuumDialogOpen,
  selectSetDatabaseVacuuming,
  selectToggleDatabaseExportType,
  selectToggleDatabaseImportType,
  useDatabasePageStore,
} from "@/stores/database";
import { useDatabaseStats, databaseKeys } from "@/hooks/api/use-database";

export function useDatabasePage() {
  const navigate = useNavigate();
  const previewRequestRef = useRef(0);
  const queryClient = useQueryClient();

  const { data: stats = null, isLoading: loading } = useDatabaseStats();

  const vacuumDialogOpen = useDatabasePageStore(selectDatabaseVacuumDialogOpen);
  const vacuuming = useDatabasePageStore(selectDatabaseVacuuming);
  const exportConfigDialogOpen = useDatabasePageStore(selectDatabaseExportConfigDialogOpen);
  const exporting = useDatabasePageStore(selectDatabaseExporting);
  const exportTypes = useDatabasePageStore(selectDatabaseExportTypes);
  const exportDatabaseDialogOpen = useDatabasePageStore(selectDatabaseExportDatabaseDialogOpen);
  const exportingDatabase = useDatabasePageStore(selectDatabaseExportingDatabase);
  const importDialogOpen = useDatabasePageStore(selectDatabaseImportDialogOpen);
  const importing = useDatabasePageStore(selectDatabaseImporting);
  const importMode = useDatabasePageStore(selectDatabaseImportMode);
  const importTypes = useDatabasePageStore(selectDatabaseImportTypes);
  const selectedFile = useDatabasePageStore(selectDatabaseSelectedFile);
  const previewData = useDatabasePageStore(selectDatabasePreviewData);
  const previewLoading = useDatabasePageStore(selectDatabasePreviewLoading);
  const previewError = useDatabasePageStore(selectDatabasePreviewError);
  const importFileInputKey = useDatabasePageStore(selectDatabaseImportFileInputKey);

  const setVacuumDialogOpen = useDatabasePageStore(selectSetDatabaseVacuumDialogOpen);
  const setVacuuming = useDatabasePageStore(selectSetDatabaseVacuuming);
  const setExportConfigDialogOpen = useDatabasePageStore(selectSetDatabaseExportConfigDialogOpen);
  const setExporting = useDatabasePageStore(selectSetDatabaseExporting);
  const toggleExportType = useDatabasePageStore(selectToggleDatabaseExportType);
  const setExportDatabaseDialogOpen = useDatabasePageStore(selectSetDatabaseExportDatabaseDialogOpen);
  const setExportingDatabase = useDatabasePageStore(selectSetDatabaseExportingDatabase);
  const setImportDialogOpen = useDatabasePageStore(selectSetDatabaseImportDialogOpen);
  const setImporting = useDatabasePageStore(selectSetDatabaseImporting);
  const setImportMode = useDatabasePageStore(selectSetDatabaseImportMode);
  const toggleImportType = useDatabasePageStore(selectToggleDatabaseImportType);
  const setSelectedFile = useDatabasePageStore(selectSetDatabaseSelectedFile);
  const setPreviewData = useDatabasePageStore(selectSetDatabasePreviewData);
  const setPreviewLoading = useDatabasePageStore(selectSetDatabasePreviewLoading);
  const setPreviewError = useDatabasePageStore(selectSetDatabasePreviewError);
  const bumpImportFileInputKey = useDatabasePageStore(selectBumpDatabaseImportFileInputKey);
  const resetTransient = useDatabasePageStore(selectResetDatabaseTransient);

  const clearImportFileSelection = useCallback(() => {
    previewRequestRef.current += 1;
    setSelectedFile(null);
    setPreviewData(null);
    setPreviewError(null);
    setPreviewLoading(false);
    bumpImportFileInputKey();
  }, [bumpImportFileInputKey, setPreviewData, setPreviewError, setPreviewLoading, setSelectedFile]);

  useEffect(() => {
    return () => {
      previewRequestRef.current += 1;
      resetTransient();
    };
  }, [resetTransient]);

  const usageRate =
    stats?.page_count && stats.page_count > 0
      ? (((stats.page_count - stats.free_pages) / stats.page_count) * 100).toFixed(1)
      : "0";

  const goBackHome = () => {
    navigate("/");
  };

  const refreshStats = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: databaseKeys.stats() });
  }, [queryClient]);

  const handleVacuum = async () => {
    setVacuumDialogOpen(false);
    setVacuuming(true);
    try {
      await vacuumDatabase();
      toast.success("数据库压缩完成");
      await queryClient.invalidateQueries({ queryKey: databaseKeys.stats() });
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
      await queryClient.invalidateQueries({ queryKey: databaseKeys.stats() });
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
