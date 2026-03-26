import { createStore } from "zustand/vanilla";
import type { DatabaseStats, ExportType } from "@/lib/api";
import { ALL_EXPORT_TYPES, type ImportMode, type ImportPreviewData } from "@/stores/database/types";

type Updater<T> = T | ((previous: T) => T);

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === "function" ? (updater as (previous: T) => T)(previous) : updater;
}

function toggleTypeSelection(types: ExportType[], type: ExportType): ExportType[] {
  return types.includes(type) ? types.filter((item) => item !== type) : [...types, type];
}

export type DatabasePageState = {
  stats: DatabaseStats | null;
  loading: boolean;

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

  setStats: (stats: DatabaseStats | null) => void;
  setLoading: (loading: boolean) => void;

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
  stats: null,
  loading: true,

  vacuumDialogOpen: false,
  vacuuming: false,

  exportConfigDialogOpen: false,
  exporting: false,
  exportTypes: [...ALL_EXPORT_TYPES],

  exportDatabaseDialogOpen: false,
  exportingDatabase: false,

  importDialogOpen: false,
  importing: false,
  importMode: "merge",
  importTypes: [...ALL_EXPORT_TYPES],
  selectedFile: null,
  previewData: null,
  previewLoading: false,
  previewError: null,
  importFileInputKey: 0,

  setStats: (stats: DatabaseStats | null) => set({ stats }),
  setLoading: (loading: boolean) => set({ loading }),

  setVacuumDialogOpen: (open: boolean) => set({ vacuumDialogOpen: open }),
  setVacuuming: (vacuuming: boolean) => set({ vacuuming }),

  setExportConfigDialogOpen: (open: boolean) => set({ exportConfigDialogOpen: open }),
  setExporting: (exporting: boolean) => set({ exporting }),
  setExportTypes: (next: Updater<ExportType[]>) =>
    set((state) => ({ exportTypes: resolveUpdater(next, state.exportTypes) })),
  toggleExportType: (type: ExportType) => {
    const current = get();
    set({ exportTypes: toggleTypeSelection(current.exportTypes, type) });
  },

  setExportDatabaseDialogOpen: (open: boolean) => set({ exportDatabaseDialogOpen: open }),
  setExportingDatabase: (exporting: boolean) => set({ exportingDatabase: exporting }),

  setImportDialogOpen: (open: boolean) => set({ importDialogOpen: open }),
  setImporting: (importing: boolean) => set({ importing }),
  setImportMode: (mode: ImportMode) => set({ importMode: mode }),
  setImportTypes: (next: Updater<ExportType[]>) =>
    set((state) => ({ importTypes: resolveUpdater(next, state.importTypes) })),
  toggleImportType: (type: ExportType) => {
    const current = get();
    set({ importTypes: toggleTypeSelection(current.importTypes, type) });
  },
  setSelectedFile: (file: File | null) => set({ selectedFile: file }),
  setPreviewData: (data: ImportPreviewData | null) => set({ previewData: data }),
  setPreviewLoading: (loading: boolean) => set({ previewLoading: loading }),
  setPreviewError: (error: string | null) => set({ previewError: error }),
  bumpImportFileInputKey: () => set((state) => ({ importFileInputKey: state.importFileInputKey + 1 })),

  resetTransient: () =>
    set({
      stats: null,
      loading: true,
      vacuumDialogOpen: false,
      vacuuming: false,
      exportConfigDialogOpen: false,
      exporting: false,
      exportTypes: [...ALL_EXPORT_TYPES],
      exportDatabaseDialogOpen: false,
      exportingDatabase: false,
      importDialogOpen: false,
      importing: false,
      importMode: "merge",
      importTypes: [...ALL_EXPORT_TYPES],
      selectedFile: null,
      previewData: null,
      previewLoading: false,
      previewError: null,
      importFileInputKey: 0,
    }),
}));

