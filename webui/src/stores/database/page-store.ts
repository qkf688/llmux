import { createStore } from "zustand/vanilla";
import type { ExportType } from "@/lib/api";
import { type Updater, resolveUpdater } from "@/stores/core/updater";
import { readDatabasePagePreferences, writeDatabasePagePreferences } from "@/stores/database/persist";
import type { DatabasePagePreferences, ImportMode, ImportPreviewData } from "@/stores/database/types";

function toggleTypeSelection(types: ExportType[], type: ExportType): ExportType[] {
  return types.includes(type) ? types.filter((item) => item !== type) : [...types, type];
}

type PreferencesState = {
  exportTypes: ExportType[];
  importMode: ImportMode;
  importTypes: ExportType[];
};

function persistPreferences(get: () => PreferencesState, next: Partial<DatabasePagePreferences>): void {
  const current = get();
  writeDatabasePagePreferences({
    exportTypes: next.exportTypes ?? current.exportTypes,
    importMode: next.importMode ?? current.importMode,
    importTypes: next.importTypes ?? current.importTypes,
  });
}

const preferences = readDatabasePagePreferences();

export type DatabasePageState = {
  vacuumDialogOpen: boolean;
  vacuuming: boolean;

  exportConfigDialogOpen: boolean;
  exporting: boolean;
  exportTypes: ExportType[];

  exportDatabaseDialogOpen: boolean;
  exportingDatabase: boolean;

  importDialogOpen: boolean;
  importing: boolean;
  importMode: ImportMode;
  importTypes: ExportType[];
  selectedFile: File | null;
  previewData: ImportPreviewData | null;
  previewLoading: boolean;
  previewError: string | null;
  importFileInputKey: number;

  setVacuumDialogOpen: (open: boolean) => void;
  setVacuuming: (vacuuming: boolean) => void;

  setExportConfigDialogOpen: (open: boolean) => void;
  setExporting: (exporting: boolean) => void;
  setExportTypes: (next: Updater<ExportType[]>) => void;
  toggleExportType: (type: ExportType) => void;

  setExportDatabaseDialogOpen: (open: boolean) => void;
  setExportingDatabase: (exporting: boolean) => void;

  setImportDialogOpen: (open: boolean) => void;
  setImporting: (importing: boolean) => void;
  setImportMode: (mode: ImportMode) => void;
  setImportTypes: (next: Updater<ExportType[]>) => void;
  toggleImportType: (type: ExportType) => void;
  setSelectedFile: (file: File | null) => void;
  setPreviewData: (data: ImportPreviewData | null) => void;
  setPreviewLoading: (loading: boolean) => void;
  setPreviewError: (error: string | null) => void;
  bumpImportFileInputKey: () => void;

  resetTransient: () => void;
};

export const databasePageStore = createStore<DatabasePageState>()((set, get) => ({
  vacuumDialogOpen: false,
  vacuuming: false,

  exportConfigDialogOpen: false,
  exporting: false,
  exportTypes: preferences.exportTypes,

  exportDatabaseDialogOpen: false,
  exportingDatabase: false,

  importDialogOpen: false,
  importing: false,
  importMode: preferences.importMode,
  importTypes: preferences.importTypes,
  selectedFile: null,
  previewData: null,
  previewLoading: false,
  previewError: null,
  importFileInputKey: 0,

  setVacuumDialogOpen: (open: boolean) => set({ vacuumDialogOpen: open }),
  setVacuuming: (vacuuming: boolean) => set({ vacuuming }),

  setExportConfigDialogOpen: (open: boolean) => set({ exportConfigDialogOpen: open }),
  setExporting: (exporting: boolean) => set({ exporting }),
  setExportTypes: (next: Updater<ExportType[]>) =>
    set((state) => {
      const resolved = resolveUpdater(next, state.exportTypes);
      persistPreferences(get, { exportTypes: resolved });
      return { exportTypes: resolved };
    }),
  toggleExportType: (type: ExportType) => {
    const current = get();
    const next = toggleTypeSelection(current.exportTypes, type);
    persistPreferences(get, { exportTypes: next });
    set({ exportTypes: next });
  },

  setExportDatabaseDialogOpen: (open: boolean) => set({ exportDatabaseDialogOpen: open }),
  setExportingDatabase: (exporting: boolean) => set({ exportingDatabase: exporting }),

  setImportDialogOpen: (open: boolean) => set({ importDialogOpen: open }),
  setImporting: (importing: boolean) => set({ importing }),
  setImportMode: (mode: ImportMode) => {
    const current = get();
    if (current.importMode === mode) return;
    persistPreferences(get, { importMode: mode });
    set({ importMode: mode });
  },
  setImportTypes: (next: Updater<ExportType[]>) =>
    set((state) => {
      const resolved = resolveUpdater(next, state.importTypes);
      persistPreferences(get, { importTypes: resolved });
      return { importTypes: resolved };
    }),
  toggleImportType: (type: ExportType) => {
    const current = get();
    const next = toggleTypeSelection(current.importTypes, type);
    persistPreferences(get, { importTypes: next });
    set({ importTypes: next });
  },
  setSelectedFile: (file: File | null) => set({ selectedFile: file }),
  setPreviewData: (data: ImportPreviewData | null) => set({ previewData: data }),
  setPreviewLoading: (loading: boolean) => set({ previewLoading: loading }),
  setPreviewError: (error: string | null) => set({ previewError: error }),
  bumpImportFileInputKey: () => set((state) => ({ importFileInputKey: state.importFileInputKey + 1 })),

  resetTransient: () =>
    set({
      vacuumDialogOpen: false,
      vacuuming: false,
      exportConfigDialogOpen: false,
      exporting: false,
      exportDatabaseDialogOpen: false,
      exportingDatabase: false,
      importDialogOpen: false,
      importing: false,
      selectedFile: null,
      previewData: null,
      previewLoading: false,
      previewError: null,
      importFileInputKey: 0,
    }),
}));
