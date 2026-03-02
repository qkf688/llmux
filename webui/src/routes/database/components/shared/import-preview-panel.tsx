import { Label } from "@/components/ui/label";
import { RefreshCw } from "lucide-react";
import type { ImportPreviewData } from "../../types";
import { PREVIEW_LABELS } from "../../types";

type ImportPreviewPanelProps = {
  visible: boolean;
  loading: boolean;
  error: string | null;
  preview: ImportPreviewData | null;
};

export function ImportPreviewPanel({ visible, loading, error, preview }: ImportPreviewPanelProps) {
  if (!visible) {
    return null;
  }

  return (
    <div className="space-y-2">
      <Label>文件内容预览</Label>
      {loading ? (
        <div className="flex items-center justify-center py-4">
          <RefreshCw className="h-5 w-5 animate-spin text-muted-foreground" />
          <span className="ml-2 text-sm text-muted-foreground">加载预览中...</span>
        </div>
      ) : error ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">❌ {error}</div>
      ) : preview ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-2 p-3 rounded-md border border-muted">
          {PREVIEW_LABELS.map((item) => (
            <div className="flex justify-between items-center" key={item.key}>
              <span className="text-sm">{item.label}</span>
              <span className="font-medium">{preview[item.key]} 条</span>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
