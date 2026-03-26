import { createStore } from "zustand/vanilla";
import type { HealthCheckLog, Model, Provider } from "@/lib/api";
import {
  readHealthCheckLogsPagePreferences,
  writeHealthCheckLogsPagePreferences,
} from "@/stores/health-check-logs/persist";
import {
  DEFAULT_HEALTH_CHECK_LOGS_FILTERS,
  HEALTH_CHECK_PAGE_SIZE_OPTIONS,
  type HealthCheckLogsFilters,
  type HealthCheckLogsPagePreferences,
} from "@/stores/health-check-logs/types";

const preferences = readHealthCheckLogsPagePreferences();

export type HealthCheckLogsPageState = {
  loading: boolean;
  logs: HealthCheckLog[];
  providers: Provider[];
  models: Model[];

  filters: HealthCheckLogsFilters;
  page: number;
  pageSize: HealthCheckLogsPagePreferences["pageSize"];
  total: number;
  pages: number;

  detailLog: HealthCheckLog | null;
  detailDialogOpen: boolean;
  clearDialogOpen: boolean;
  clearingLogs: boolean;

  resultDialogOpen: boolean;
  currentBatchId: string | null;
  backgroundBatchId: string | null;
  backgroundCheckComplete: boolean;

  setLoading: (loading: boolean) => void;
  setLogs: (logs: HealthCheckLog[]) => void;
  setProviders: (providers: Provider[]) => void;
  setModels: (models: Model[]) => void;

  setFilter: (key: keyof HealthCheckLogsFilters, value: string) => void;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;
  setTotal: (total: number) => void;
  setPages: (pages: number) => void;

  openDetailDialog: (log: HealthCheckLog) => void;
  setDetailDialogOpen: (open: boolean) => void;

  setClearDialogOpen: (open: boolean) => void;
  setClearingLogs: (clearing: boolean) => void;

  setResultDialogOpen: (open: boolean) => void;
  setCurrentBatchId: (batchId: string | null) => void;
  setBackgroundBatchId: (batchId: string | null) => void;
  setBackgroundCheckComplete: (completed: boolean) => void;

  resetTransient: () => void;
};

function persistPreferences(get: () => HealthCheckLogsPageState, next: Partial<HealthCheckLogsPagePreferences>): void {
  const current = get();
  writeHealthCheckLogsPagePreferences({
    filters: next.filters ?? current.filters,
    pageSize: next.pageSize ?? current.pageSize,
  });
}

export const healthCheckLogsPageStore = createStore<HealthCheckLogsPageState>()((set, get) => ({
  loading: true,
  logs: [],
  providers: [],
  models: [],

  filters: preferences.filters ?? { ...DEFAULT_HEALTH_CHECK_LOGS_FILTERS },
  page: 1,
  pageSize: preferences.pageSize,
  total: 0,
  pages: 0,

  detailLog: null,
  detailDialogOpen: false,
  clearDialogOpen: false,
  clearingLogs: false,

  resultDialogOpen: false,
  currentBatchId: null,
  backgroundBatchId: null,
  backgroundCheckComplete: false,

  setLoading: (loading: boolean) => set({ loading }),
  setLogs: (logs: HealthCheckLog[]) => set({ logs }),
  setProviders: (providers: Provider[]) => set({ providers }),
  setModels: (models: Model[]) => set({ models }),

  setFilter: (key: keyof HealthCheckLogsFilters, value: string) => {
    const current = get();
    if (current.filters[key] === value) return;

    const nextFilters = { ...current.filters, [key]: value };
    persistPreferences(get, { filters: nextFilters });
    set({ filters: nextFilters, page: 1 });
  },
  setPage: (page: number) => {
    const current = get();
    if (current.page === page) return;
    set({ page });
  },
  setPageSize: (size: number) => {
    const current = get();
    if (size === current.pageSize) return;

    const validated = HEALTH_CHECK_PAGE_SIZE_OPTIONS.includes(size as (typeof HEALTH_CHECK_PAGE_SIZE_OPTIONS)[number])
      ? (size as (typeof HEALTH_CHECK_PAGE_SIZE_OPTIONS)[number])
      : 20;

    persistPreferences(get, { pageSize: validated });
    set({ pageSize: validated, page: 1 });
  },
  setTotal: (total: number) => set({ total }),
  setPages: (pages: number) => set({ pages }),

  openDetailDialog: (log: HealthCheckLog) => set({ detailLog: log, detailDialogOpen: true }),
  setDetailDialogOpen: (open: boolean) => set(open ? { detailDialogOpen: true } : { detailDialogOpen: false, detailLog: null }),

  setClearDialogOpen: (open: boolean) => set({ clearDialogOpen: open }),
  setClearingLogs: (clearing: boolean) => set({ clearingLogs: clearing }),

  setResultDialogOpen: (open: boolean) => set({ resultDialogOpen: open }),
  setCurrentBatchId: (batchId: string | null) => set({ currentBatchId: batchId }),
  setBackgroundBatchId: (batchId: string | null) => set({ backgroundBatchId: batchId }),
  setBackgroundCheckComplete: (completed: boolean) => set({ backgroundCheckComplete: completed }),

  resetTransient: () =>
    set({
      loading: true,
      logs: [],
      providers: [],
      models: [],
      page: 1,
      total: 0,
      pages: 0,
      detailLog: null,
      detailDialogOpen: false,
      clearDialogOpen: false,
      clearingLogs: false,
      resultDialogOpen: false,
      currentBatchId: null,
      backgroundBatchId: null,
      backgroundCheckComplete: false,
    }),
}));
