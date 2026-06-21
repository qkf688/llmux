import { createStore } from "zustand/vanilla";
import type { HealthCheckLog } from "@/lib/api";
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
  filters: HealthCheckLogsFilters;
  page: number;
  pageSize: HealthCheckLogsPagePreferences["pageSize"];

  detailLog: HealthCheckLog | null;
  detailDialogOpen: boolean;
  clearDialogOpen: boolean;

  resultDialogOpen: boolean;
  currentBatchId: string | null;
  backgroundBatchId: string | null;
  backgroundCheckComplete: boolean;

  setFilter: (key: keyof HealthCheckLogsFilters, value: string) => void;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;

  openDetailDialog: (log: HealthCheckLog) => void;
  setDetailDialogOpen: (open: boolean) => void;

  setClearDialogOpen: (open: boolean) => void;

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
  filters: preferences.filters ?? { ...DEFAULT_HEALTH_CHECK_LOGS_FILTERS },
  page: 1,
  pageSize: preferences.pageSize,

  detailLog: null,
  detailDialogOpen: false,
  clearDialogOpen: false,

  resultDialogOpen: false,
  currentBatchId: null,
  backgroundBatchId: null,
  backgroundCheckComplete: false,

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

  openDetailDialog: (log: HealthCheckLog) => set({ detailLog: log, detailDialogOpen: true }),
  setDetailDialogOpen: (open: boolean) => set(open ? { detailDialogOpen: true } : { detailDialogOpen: false, detailLog: null }),

  setClearDialogOpen: (open: boolean) => set({ clearDialogOpen: open }),

  setResultDialogOpen: (open: boolean) => set({ resultDialogOpen: open }),
  setCurrentBatchId: (batchId: string | null) => set({ currentBatchId: batchId }),
  setBackgroundBatchId: (batchId: string | null) => set({ backgroundBatchId: batchId }),
  setBackgroundCheckComplete: (completed: boolean) => set({ backgroundCheckComplete: completed }),

  resetTransient: () =>
    set({
      page: 1,
      detailLog: null,
      detailDialogOpen: false,
      clearDialogOpen: false,
      resultDialogOpen: false,
      currentBatchId: null,
      backgroundBatchId: null,
      backgroundCheckComplete: false,
    }),
}));
