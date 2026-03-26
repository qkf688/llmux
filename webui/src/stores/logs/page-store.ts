import { createStore } from "zustand/vanilla";
import type { ChatLog, Model, Provider } from "@/lib/api";
import { readLogsPagePreferences, writeLogsPagePreferences } from "@/stores/logs/persist";
import { type LogsFilters } from "@/stores/logs/types";

export type LogsPageState = {
  loading: boolean;
  logs: ChatLog[];
  providers: Provider[];
  models: Model[];
  userAgents: string[];
  availableStyles: string[];

  filters: LogsFilters;
  page: number;
  pageSize: number;
  total: number;
  pages: number;

  selectedIds: Set<number>;
  selectedLog: ChatLog | null;

  detailDialogOpen: boolean;
  deleteDialogOpen: boolean;
  batchDeleteDialogOpen: boolean;
  clearAllDialogOpen: boolean;
  clearFilteredDialogOpen: boolean;
  logToDelete: number | null;

  isDeleting: boolean;
  isClearingAll: boolean;
  isClearingFiltered: boolean;

  setLoading: (loading: boolean) => void;
  setLogs: (logs: ChatLog[]) => void;
  setProviders: (providers: Provider[]) => void;
  setModels: (models: Model[]) => void;
  setUserAgents: (agents: string[]) => void;
  setAvailableStyles: (styles: string[]) => void;
  setTotal: (total: number) => void;
  setPages: (pages: number) => void;

  setFilter: (key: keyof LogsFilters, value: string) => void;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;

  setSelectedIds: (selectedIds: Set<number>) => void;
  clearSelection: () => void;

  openDetailDialog: (log: ChatLog) => void;
  setDetailDialogOpen: (open: boolean) => void;

  openDeleteDialog: (id: number) => void;
  setDeleteDialogOpen: (open: boolean) => void;

  setIsDeleting: (deleting: boolean) => void;
  setIsClearingAll: (clearing: boolean) => void;
  setIsClearingFiltered: (clearing: boolean) => void;

  setBatchDeleteDialogOpen: (open: boolean) => void;
  setClearAllDialogOpen: (open: boolean) => void;
  setClearFilteredDialogOpen: (open: boolean) => void;

  resetTransient: () => void;
};

const preferences = readLogsPagePreferences();

export const logsPageStore = createStore<LogsPageState>()((set, get) => ({
  loading: true,
  logs: [],
  providers: [],
  models: [],
  userAgents: [],
  availableStyles: [],

  filters: preferences.filters,
  page: 1,
  pageSize: preferences.pageSize,
  total: 0,
  pages: 0,

  selectedIds: new Set<number>(),
  selectedLog: null,

  detailDialogOpen: false,
  deleteDialogOpen: false,
  batchDeleteDialogOpen: false,
  clearAllDialogOpen: false,
  clearFilteredDialogOpen: false,
  logToDelete: null,

  isDeleting: false,
  isClearingAll: false,
  isClearingFiltered: false,

  setLoading: (loading: boolean) => set({ loading }),
  setLogs: (logs: ChatLog[]) => set({ logs }),
  setProviders: (providers: Provider[]) => set({ providers }),
  setModels: (models: Model[]) => set({ models }),
  setUserAgents: (agents: string[]) => set({ userAgents: agents }),
  setAvailableStyles: (styles: string[]) => set({ availableStyles: styles }),
  setTotal: (total: number) => set({ total }),
  setPages: (pages: number) => set({ pages }),

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

  setIsDeleting: (deleting: boolean) => set({ isDeleting: deleting }),
  setIsClearingAll: (clearing: boolean) => set({ isClearingAll: clearing }),
  setIsClearingFiltered: (clearing: boolean) => set({ isClearingFiltered: clearing }),

  setBatchDeleteDialogOpen: (open: boolean) => set({ batchDeleteDialogOpen: open }),
  setClearAllDialogOpen: (open: boolean) => set({ clearAllDialogOpen: open }),
  setClearFilteredDialogOpen: (open: boolean) => set({ clearFilteredDialogOpen: open }),

  resetTransient: () =>
    set({
      loading: true,
      logs: [],
      providers: [],
      models: [],
      userAgents: [],
      availableStyles: [],
      total: 0,
      pages: 0,
      selectedIds: new Set(),
      selectedLog: null,
      detailDialogOpen: false,
      deleteDialogOpen: false,
      batchDeleteDialogOpen: false,
      clearAllDialogOpen: false,
      clearFilteredDialogOpen: false,
      logToDelete: null,
      isDeleting: false,
      isClearingAll: false,
      isClearingFiltered: false,
    }),
}));
