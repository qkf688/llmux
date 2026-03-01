import { apiRequest } from "../../core/client";

export interface TableStat {
  name: string;
  display_name: string;
  count: number;
  estimated_size_human: string;
}

export interface DatabaseStats {
  file_path: string;
  file_size: number;
  file_size_human: string;
  table_stats: TableStat[];
  page_count: number;
  page_size: number;
  free_pages: number;
  last_vacuum_at?: string;
  can_vacuum: boolean;
  db_path: string;
  sqlite_version: string;
  encoding: string;
  last_modified: string;
}

function toSafeNumber(value: unknown, fallback = 0): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return fallback;
}

function toSafeString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }

  if (unitIndex === 0) {
    return `${Math.round(value)} ${units[unitIndex]}`;
  }

  return `${value.toFixed(value >= 10 ? 1 : 2)} ${units[unitIndex]}`;
}

function normalizeTableStats(raw: unknown, source: Record<string, unknown>): TableStat[] {
  if (Array.isArray(raw)) {
    return raw
      .map((item): TableStat | null => {
        if (!item || typeof item !== "object") {
          return null;
        }
        const table = item as Record<string, unknown>;
        const name = toSafeString(table.name);
        if (!name) {
          return null;
        }
        return {
          name,
          display_name: toSafeString(table.display_name, name),
          count: toSafeNumber(table.count),
          estimated_size_human: toSafeString(table.estimated_size_human, "-"),
        };
      })
      .filter((item): item is TableStat => item !== null);
  }

  const hasLegacyTableCounts =
    "provider_count" in source || "model_count" in source || "model_provider_count" in source;
  if (!hasLegacyTableCounts) {
    return [];
  }

  return [
    {
      name: "providers",
      display_name: "providers",
      count: toSafeNumber(source.provider_count),
      estimated_size_human: "-",
    },
    {
      name: "models",
      display_name: "models",
      count: toSafeNumber(source.model_count),
      estimated_size_human: "-",
    },
    {
      name: "model_with_providers",
      display_name: "model_with_providers",
      count: toSafeNumber(source.model_provider_count),
      estimated_size_human: "-",
    },
  ];
}

function normalizeDatabaseStats(raw: Record<string, unknown>): DatabaseStats {
  const filePath = toSafeString(raw.file_path, toSafeString(raw.database_path));
  const dbPath = toSafeString(raw.db_path, filePath);
  const fileSize = toSafeNumber(raw.file_size, toSafeNumber(raw.database_size));
  const pageCount = toSafeNumber(raw.page_count);
  const pageSize = toSafeNumber(raw.page_size);
  const freePages = toSafeNumber(raw.free_pages);

  return {
    file_path: filePath,
    file_size: fileSize,
    file_size_human: toSafeString(raw.file_size_human, formatBytes(fileSize)),
    table_stats: normalizeTableStats(raw.table_stats, raw),
    page_count: pageCount,
    page_size: pageSize,
    free_pages: freePages,
    last_vacuum_at: toSafeString(raw.last_vacuum_at) || undefined,
    can_vacuum: typeof raw.can_vacuum === "boolean" ? raw.can_vacuum : true,
    db_path: dbPath,
    sqlite_version: toSafeString(raw.sqlite_version, "-"),
    encoding: toSafeString(raw.encoding, "-"),
    last_modified: toSafeString(raw.last_modified, ""),
  };
}

export async function getDatabaseStats(): Promise<DatabaseStats> {
  const raw = await apiRequest<Record<string, unknown>>("/system/database-stats");
  return normalizeDatabaseStats(raw);
}

export async function vacuumDatabase(): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/maintenance/vacuum", {
    method: "POST",
  });
}
