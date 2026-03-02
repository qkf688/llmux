import type { ImportConfigResponse } from "@/lib/api";

type ImportSummaryItem = {
  imported: number;
  skipped: number;
  label: string;
};

function collectImportSummary(result: ImportConfigResponse): ImportSummaryItem[] {
  return [
    { label: "提供商", imported: result.providers.imported, skipped: result.providers.skipped },
    { label: "模型", imported: result.models.imported, skipped: result.models.skipped },
    { label: "关联", imported: result.associations.imported, skipped: result.associations.skipped },
    { label: "模板", imported: result.templates.imported, skipped: result.templates.skipped },
    { label: "设置", imported: result.settings.imported, skipped: result.settings.skipped },
  ];
}

export function buildImportResultMessage(result: ImportConfigResponse): string {
  const summary = collectImportSummary(result);
  const importedParts = summary
    .filter((item) => item.imported > 0)
    .map((item) => `${item.label}: ${item.imported} 条`);
  const skippedCount = summary.reduce((total, item) => total + item.skipped, 0);
  const importedText = importedParts.length > 0 ? importedParts.join(", ") : "0";

  return `导入成功！已导入: ${importedText}${
    skippedCount > 0 ? `，跳过: ${skippedCount} 条` : ""
  }`;
}
