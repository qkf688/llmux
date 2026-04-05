import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Download } from "lucide-react";
import type { ChatLogExportSectionKey, ChatLogExportSections } from "../../../utils/export-log";

const EXPORT_SECTION_OPTIONS: Array<{
  key: ChatLogExportSectionKey;
  label: string;
  description: string;
}> = [
  { key: "basic", label: "基本信息", description: "模型/提供商/状态/UA/IP/重试/格式转换等" },
  { key: "error", label: "错误信息", description: "仅当该字段存在时导出" },
  { key: "performance", label: "性能指标", description: "代理耗时/首包耗时/完成耗时/TPS" },
  { key: "tokens", label: "Token 使用", description: "prompt/completion/total 及 details" },
  { key: "request", label: "请求内容", description: "请求头/请求体（需要加载详情）" },
  { key: "response", label: "响应内容", description: "响应头/响应体/原始响应体（需要加载详情）" },
];

type ExportLogDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  exporting: boolean;
  sections: ChatLogExportSections;
  onToggleSection: (key: ChatLogExportSectionKey) => void;
  onConfirm: () => void;
};

export function ExportLogDialog({
  open,
  onOpenChange,
  exporting,
  sections,
  onToggleSection,
  onConfirm,
}: ExportLogDialogProps) {
  const selectedCount = Object.values(sections).filter(Boolean).length;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>导出日志</DialogTitle>
          <DialogDescription>选择要导出的内容（默认全选）</DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-3">
          {EXPORT_SECTION_OPTIONS.map((option) => {
            const inputId = `log-export-${option.key}`;
            return (
              <div className="flex items-start space-x-2" key={option.key}>
                <Checkbox
                  id={inputId}
                  checked={sections[option.key]}
                  onCheckedChange={() => onToggleSection(option.key)}
                  disabled={exporting}
                />
                <Label htmlFor={inputId} className="cursor-pointer leading-tight">
                  <div className="text-sm">{option.label}</div>
                  <div className="text-xs text-muted-foreground">{option.description}</div>
                </Label>
              </div>
            );
          })}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={exporting}>
            取消
          </Button>
          <Button onClick={onConfirm} disabled={exporting || selectedCount === 0} className="gap-2">
            <Download className="h-4 w-4" />
            {exporting ? "导出中..." : "导出"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

