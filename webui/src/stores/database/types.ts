import type { ExportType } from "@/lib/api";

export type ImportMode = "merge" | "replace";

export interface ImportPreviewData {
  providers: number;
  models: number;
  associations: number;
  templates: number;
  settings: number;
}

export const ALL_EXPORT_TYPES: ExportType[] = ["providers", "models", "associations", "templates", "settings"];

export type DatabasePagePreferences = {
  exportTypes: ExportType[];
  importMode: ImportMode;
  importTypes: ExportType[];
};

export const DEFAULT_DATABASE_PAGE_PREFERENCES: DatabasePagePreferences = {
  exportTypes: [...ALL_EXPORT_TYPES],
  importMode: "merge",
  importTypes: [...ALL_EXPORT_TYPES],
};
