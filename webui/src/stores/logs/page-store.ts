import { createStore } from "zustand/vanilla";
import type { ChatLog } from "@/lib/api";
import { readLogsPagePreferences, writeLogsPagePreferences } from "@/stores/logs/persist";
import { type LogsFilters } from "@/stores/logs/types";

export type LogsPageState = {
  filters: LogsFilters;
  page: number;
  pageSize: number;

  selectedIds: Set<number>;
  selectedLog: ChatLog | null;

  detailDialogOpen: boolean;
  deleteDialogOpen: boolean;
  batchDeleteDialogOpen: boolean;
  clearAllDialogOpen: boolean;
  clearFilteredDialogOpen: boolean;
  logToDelete: number | null;

  setFilter: (key: keyof LogsFilters, value: string) => void;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;

  setSelectedIds: (selectedIds: Set<number>) => void;
  clearSelection: () => void;

  openDetailDialog: (log: ChatLog) => void;
  setDetailDialogOpen: (open: boolean) => void;

  openDeleteDialog: (id: number) => void;
  setDeleteDialogOpen: (open: boolean) => void;

  setBatchDeleteDialogOpen: (open: boolean) => void;
  setClearAllDialogOpen: (open: boolean) => void;
  setClearFilteredDialogOpen: (open: boolean) => void;

  resetTransient: () => void;
};

const preferences = readLogsPagePreferences();

export const logsPageStore = createStore<LogsPageState>()((set, get) => ({
  filters: preferences.filters,
  page: 1,
  pageSize: preferences.pageSize,

  selectedIds: new Set<number>(),
  selectedLog: null,

  detailDialogOpen: false,
  deleteDialogOpen: false,
  batchDeleteDialogOpen: false,
  clearAllDialogOpen: false,
  clearFilteredDialogOpen: false,
  logToDelete: null,

  setFilter: (key: keyof LogsFilters, value: string) => {
    const current = get();
    if (current.filters[key] === value) return;

    const nextFilters = { ...current.filters, [key]: value };
    writeLogsPagePreferences({ filters: nextFilters, pageSize: current.pageSize });

    set({
      filters: nextFilters,
      page: 1,
      selectedIds: new Set(),
    });
  },
  setPage: (page: number) => {
    const current = get();
    if (current.page === page) return;
    set({ page, selectedIds: new Set() });
  },
  setPageSize: (size: number) => {
    const current = get();
    if (current.pageSize === size) return;
    writeLogsPagePreferences({ filters: current.filters, pageSize: size });
    set({ pageSize: size, page: 1, selectedIds: new Set() });
  },

  setSelectedIds: (selectedIds: Set<number>) => set({ selectedIds }),
  clearSelection: () => set({ selectedIds: new Set() }),

  openDetailDialog: (log: ChatLog) => set({ selectedLog: log, detailDialogOpen: true }),
  setDetailDialogOpen: (open: boolean) => set(open ? { detailDialogOpen: true } : { detailDialogOpen: false, selectedLog: null }),

  openDeleteDialog: (id: number) => set({ logToDelete: id, deleteDialogOpen: true }),
  setDeleteDialogOpen: (open: boolean) => set(open ? { deleteDialogOpen: true } : { deleteDialogOpen: false, logToDelete: null }),

  setBatchDeleteDialogOpen: (open: boolean) => set({ batchDeleteDialogOpen: open }),
  setClearAllDialogOpen: (open: boolean) => set({ clearAllDialogOpen: open }),
  setClearFilteredDialogOpen: (open: boolean) => set({ clearFilteredDialogOpen: open }),

  resetTransient: () =>
    set({
      selectedIds: new Set(),
      selectedLog: null,
      detailDialogOpen: false,
      deleteDialogOpen: false,
      batchDeleteDialogOpen: false,
      clearAllDialogOpen: false,
      clearFilteredDialogOpen: false,
      logToDelete: null,
    }),
}));
