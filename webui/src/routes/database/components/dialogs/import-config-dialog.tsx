import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Upload } from "lucide-react";
import type { ExportType } from "@/lib/api";
import type { ImportMode, ImportPreviewData } from "../../types";
import { ExportTypeCheckboxGroup } from "../shared/export-type-checkbox-group";
import { ImportPreviewPanel } from "../shared/import-preview-panel";

type ImportConfigDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  importing: boolean;
  mode: ImportMode;
  selectedFile: File | null;
  importTypes: ExportType[];
  previewLoading: boolean;
  previewError: string | null;
  previewData: ImportPreviewData | null;
  inputKey: number;
  onModeChange: (mode: ImportMode) => void;
  onToggleType: (type: ExportType) => void;
  onFileChange: (file: File | null) => void;
  onConfirm: () => void;
};

export function ImportConfigDialog({
  open,
  onOpenChange,
  importing,
  mode,
  selectedFile,
  importTypes,
  previewLoading,
  previewError,
  previewData,
  inputKey,
  onModeChange,
  onToggleType,
  onFileChange,
  onConfirm,
}: ImportConfigDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>导入配置</DialogTitle>
          <DialogDescription>选择配置文件和导入模式，系统将根据您的选择导入数据</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label>选择配置文件</Label>
            <input
              key={inputKey}
              type="file"
              accept=".json"
              onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            />
            {selectedFile && (
              <p className="text-sm text-muted-foreground">已选择: {selectedFile.name}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label>导入模式</Label>
            <RadioGroup value={mode} onValueChange={(value) => onModeChange(value as ImportMode)}>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="merge" id="mode-merge" />
                <Label htmlFor="mode-merge" className="cursor-pointer font-normal">
                  合并模式 - 保留现有数据，仅添加新数据
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="replace" id="mode-replace" />
                <Label htmlFor="mode-replace" className="cursor-pointer font-normal text-destructive">
                  覆盖模式 - 清空现有数据，完全替换（危险操作）
                </Label>
              </div>
            </RadioGroup>
          </div>

          <div className="space-y-2">
            <Label>选择要导入的数据类型</Label>
            <ExportTypeCheckboxGroup
              prefix="import"
              selectedTypes={importTypes}
              onToggleType={onToggleType}
            />
          </div>

          <ImportPreviewPanel
            visible={selectedFile !== null}
            loading={previewLoading}
            error={previewError}
            preview={previewData}
          />

          {mode === "replace" && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              ⚠️ 警告：覆盖模式将删除所选类型的所有现有数据！请确保已备份重要数据。
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={importing}>
            取消
          </Button>
          <Button
            onClick={onConfirm}
            disabled={importing || !selectedFile || importTypes.length === 0}
            className="gap-2"
            variant={mode === "replace" ? "destructive" : "default"}
          >
            <Upload className="h-4 w-4" />
            {importing ? "导入中..." : "导入"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
