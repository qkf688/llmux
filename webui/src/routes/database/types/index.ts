import type { ExportType } from "@/lib/api";

export type ImportMode = "merge" | "replace";

export interface ImportPreviewData {
  providers: number;
  models: number;
  associations: number;
  templates: number;
  settings: number;
}

export const ALL_EXPORT_TYPES: ExportType[] = [
  "providers",
  "models",
  "associations",
  "templates",
  "settings",
];

export const EXPORT_TYPE_OPTIONS: Array<{ type: ExportType; label: string }> = [
  { type: "providers", label: "提供商配置" },
  { type: "models", label: "模型配置" },
  { type: "associations", label: "模型关联" },
  { type: "templates", label: "模型模板" },
  { type: "settings", label: "系统设置" },
];

export const PREVIEW_LABELS: Array<{ key: keyof ImportPreviewData; label: string }> = [
  { key: "providers", label: "提供商配置" },
  { key: "models", label: "模型配置" },
  { key: "associations", label: "模型关联" },
  { key: "templates", label: "模型模板" },
  { key: "settings", label: "系统设置" },
];
