import type { DatabasePageState } from "@/stores/database/page-store";

export const selectDatabaseVacuumDialogOpen = (state: DatabasePageState) => state.vacuumDialogOpen;
export const selectDatabaseVacuuming = (state: DatabasePageState) => state.vacuuming;

export const selectDatabaseExportConfigDialogOpen = (state: DatabasePageState) => state.exportConfigDialogOpen;
export const selectDatabaseExporting = (state: DatabasePageState) => state.exporting;
export const selectDatabaseExportTypes = (state: DatabasePageState) => state.exportTypes;

export const selectDatabaseExportDatabaseDialogOpen = (state: DatabasePageState) => state.exportDatabaseDialogOpen;
export const selectDatabaseExportingDatabase = (state: DatabasePageState) => state.exportingDatabase;

export const selectDatabaseImportDialogOpen = (state: DatabasePageState) => state.importDialogOpen;
export const selectDatabaseImporting = (state: DatabasePageState) => state.importing;
export const selectDatabaseImportMode = (state: DatabasePageState) => state.importMode;
export const selectDatabaseImportTypes = (state: DatabasePageState) => state.importTypes;
export const selectDatabaseSelectedFile = (state: DatabasePageState) => state.selectedFile;
export const selectDatabasePreviewData = (state: DatabasePageState) => state.previewData;
export const selectDatabasePreviewLoading = (state: DatabasePageState) => state.previewLoading;
export const selectDatabasePreviewError = (state: DatabasePageState) => state.previewError;
export const selectDatabaseImportFileInputKey = (state: DatabasePageState) => state.importFileInputKey;

export const selectSetDatabaseVacuumDialogOpen = (state: DatabasePageState) => state.setVacuumDialogOpen;
export const selectSetDatabaseVacuuming = (state: DatabasePageState) => state.setVacuuming;

export const selectSetDatabaseExportConfigDialogOpen = (state: DatabasePageState) => state.setExportConfigDialogOpen;
export const selectSetDatabaseExporting = (state: DatabasePageState) => state.setExporting;
export const selectToggleDatabaseExportType = (state: DatabasePageState) => state.toggleExportType;

export const selectSetDatabaseExportDatabaseDialogOpen = (state: DatabasePageState) => state.setExportDatabaseDialogOpen;
export const selectSetDatabaseExportingDatabase = (state: DatabasePageState) => state.setExportingDatabase;

export const selectSetDatabaseImportDialogOpen = (state: DatabasePageState) => state.setImportDialogOpen;
export const selectSetDatabaseImporting = (state: DatabasePageState) => state.setImporting;
export const selectSetDatabaseImportMode = (state: DatabasePageState) => state.setImportMode;
export const selectToggleDatabaseImportType = (state: DatabasePageState) => state.toggleImportType;
export const selectSetDatabaseSelectedFile = (state: DatabasePageState) => state.setSelectedFile;
export const selectSetDatabasePreviewData = (state: DatabasePageState) => state.setPreviewData;
export const selectSetDatabasePreviewLoading = (state: DatabasePageState) => state.setPreviewLoading;
export const selectSetDatabasePreviewError = (state: DatabasePageState) => state.setPreviewError;
export const selectBumpDatabaseImportFileInputKey = (state: DatabasePageState) => state.bumpImportFileInputKey;

export const selectResetDatabaseTransient = (state: DatabasePageState) => state.resetTransient;

