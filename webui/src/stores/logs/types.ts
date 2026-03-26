export interface LogsFilters {
  model: string;
  providerName: string;
  status: string;
  style: string;
  userAgent: string;
}

export const DEFAULT_LOGS_FILTERS: LogsFilters = {
  model: "all",
  providerName: "all",
  status: "all",
  style: "all",
  userAgent: "all",
};

export const PAGE_SIZE_OPTIONS = [10, 20, 50] as const;

export type LogsPagePreferences = {
  filters: LogsFilters;
  pageSize: number;
};

