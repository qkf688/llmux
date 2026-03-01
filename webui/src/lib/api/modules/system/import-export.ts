import { fetchWithAuth } from "../../core/client";
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

  const response = await fetchWithAuth(`/system/export-config?${params.toString()}`, {
    method: "GET",
  });

  if (!response.ok) {
    throw new Error(`Export failed: ${response.status} ${response.statusText}`);
  }

  const fallbackName = `llmio-config-${types.join("-")}-${new Date()
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
  const response = await fetchWithAuth("/system/export-database", {
    method: "GET",
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "导出数据库失败" }));
    throw new Error(error.error || "导出数据库失败");
  }

  const filename = extractFilenameFromContentDisposition(
    response.headers.get("Content-Disposition"),
    "llmio_backup.db"
  );

  const blob = await response.blob();
  downloadBlob(blob, filename);
}

export async function importConfig(options: ImportOptions): Promise<ImportConfigResponse> {
  const formData = new FormData();
  formData.append("file", options.file);
  formData.append("mode", options.mode);
  formData.append("types", options.types.join(","));

  const response = await fetchWithAuth("/system/import-config", {
    method: "POST",
    body: formData,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Import failed: ${errorText}`);
  }

  const payload = (await response.json()) as { code: number; message: string; data: ImportConfigResponse };
  if (payload.code !== 200) {
    throw new Error(`${payload.message}`);
  }

  return payload.data;
}
