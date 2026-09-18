import { apiRequestFile, apiRequestForm } from "../../core/client";
import { downloadBlob, extractFilenameFromContentDisposition } from "../../core/download";

export type ExportType = "providers" | "models" | "associations" | "templates" | "settings";

export interface ExportOptions {
  types: ExportType[];
}

export interface ImportOptions {
  mode: "merge" | "replace";
  types: ExportType[];
  file: File;
}

export interface ImportResult {
  imported: number;
  skipped: number;
  errors?: string[];
}

export interface ImportConfigResponse {
  providers: ImportResult;
  models: ImportResult;
  associations: ImportResult;
  templates: ImportResult;
  settings: ImportResult;
  total_time: string;
}

export async function exportConfig(types: ExportType[]): Promise<void> {
  const params = new URLSearchParams();
  params.append("types", types.join(","));

  const response = await apiRequestFile(`/system/export-config?${params.toString()}`, {
    method: "GET",
  });

  const fallbackName = `llmux-config-${types.join("-")}-${new Date()
    .toISOString()
    .slice(0, 19)
    .replace(/:/g, "-")}.json`;
  const filename = extractFilenameFromContentDisposition(
    response.headers.get("Content-Disposition"),
    fallbackName
  );

  const blob = await response.blob();
  downloadBlob(blob, filename);
}

export async function exportDatabase(): Promise<void> {
  const response = await apiRequestFile("/system/export-database", {
    method: "GET",
  });

  const filename = extractFilenameFromContentDisposition(
    response.headers.get("Content-Disposition"),
    "llmux_backup.db"
  );

  const blob = await response.blob();
  downloadBlob(blob, filename);
}

export async function importConfig(options: ImportOptions): Promise<ImportConfigResponse> {
  const formData = new FormData();
  formData.append("file", options.file);
  formData.append("mode", options.mode);
  formData.append("types", options.types.join(","));

  return apiRequestForm<ImportConfigResponse>("/system/import-config", {
    method: "POST",
    body: formData,
  });
}
