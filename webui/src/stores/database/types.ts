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

