import type { ImportPreviewData } from "../types";

type RawImportConfig = {
  providers?: unknown;
  models?: unknown;
  model_with_providers?: unknown;
  model_template_items?: unknown;
  settings?: unknown;
};

function toArrayLength(value: unknown): number {
  return Array.isArray(value) ? value.length : 0;
}

export async function buildImportPreview(file: File): Promise<ImportPreviewData> {
  const content = await file.text();
  const data = JSON.parse(content) as RawImportConfig;

  return {
    providers: toArrayLength(data.providers),
    models: toArrayLength(data.models),
    associations: toArrayLength(data.model_with_providers),
    templates: toArrayLength(data.model_template_items),
    settings: toArrayLength(data.settings),
  };
}
