import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Download } from "lucide-react";
import type { ExportType } from "@/lib/api";
import { ExportTypeCheckboxGroup } from "../shared/export-type-checkbox-group";

type ExportConfigDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  exporting: boolean;
  exportTypes: ExportType[];
  onToggleType: (type: ExportType) => void;
  onConfirm: () => void;
};

export function ExportConfigDialog({
  open,
  onOpenChange,
  exporting,
  exportTypes,
  onToggleType,
  onConfirm,
}: ExportConfigDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>导出配置</DialogTitle>
          <DialogDescription>选择要导出的数据类型，系统将生成 JSON 格式的配置文件</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <ExportTypeCheckboxGroup
            prefix="export"
            selectedTypes={exportTypes}
            onToggleType={onToggleType}
          />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={exporting}>
            取消
          </Button>
          <Button
            onClick={onConfirm}
            disabled={exporting || exportTypes.length === 0}
            className="gap-2"
          >
            <Download className="h-4 w-4" />
            {exporting ? "导出中..." : "导出"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
