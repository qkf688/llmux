import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import type { BatchImportData } from "@/lib/api";

const ROW_STATUS_LABEL: Record<string, string> = {
  imported: "已导入",
  skipped: "跳过",
  failed: "失败",
};

const REASON_LABEL: Record<string, string> = {
  duplicate_in_batch: "批内重复",
  duplicate_in_pool: "池内已存在",
  empty: "空行",
  encrypt_failed: "加密失败",
  db_failed: "写入失败",
};

type ImportCredentialsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 父组件拆分 keys 后调 API，返回逐行回显 */
  onImport: (text: string) => Promise<BatchImportData | null>;
  isImporting?: boolean;
};

export function ImportCredentialsDialog({
  open,
  onOpenChange,
  onImport,
  isImporting = false,
}: ImportCredentialsDialogProps) {
  const [text, setText] = useState("");
  const [result, setResult] = useState<BatchImportData | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (open) {
      setText("");
      setResult(null);
      textareaRef.current?.focus();
    }
  }, [open]);

  const handleSubmit = async () => {
    const data = await onImport(text);
    if (!data) return;
    setResult(data);
    setText("");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>添加凭据</DialogTitle>
          <DialogDescription>
            粘贴 Key，每行一个（逗号分隔亦可）；只填 1 个也合法。自动去重并加密落库。
          </DialogDescription>
        </DialogHeader>

        <DialogBody className="-mx-1 min-w-0 space-y-3 px-1">
          <Textarea
            ref={textareaRef}
            rows={8}
            className="font-mono text-xs"
            placeholder={"sk-...\nsk-...\nsk-..."}
            value={text}
            onChange={(e) => setText(e.target.value)}
            disabled={isImporting}
          />
          {result && (
            <div className="space-y-2 rounded-md border bg-muted/20 p-2 text-xs">
              <p className="text-muted-foreground">
                共 {result.total} 条：导入 {result.imported}，跳过 {result.skipped}
                {result.failed > 0 ? `，失败 ${result.failed}` : ""}
              </p>
              {result.rows.length > 0 && (
                <ul className="max-h-40 space-y-1 overflow-y-auto font-mono">
                  {result.rows.map((row) => (
                    <li key={row.Index} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                      <span className="text-muted-foreground">#{row.Index + 1}</span>
                      <span className="truncate">{row.Key || "(空)"}</span>
                      <span
                        className={
                          row.Status === "imported"
                            ? "text-success"
                            : row.Status === "failed"
                              ? "text-destructive"
                              : "text-muted-foreground"
                        }
                      >
                        {ROW_STATUS_LABEL[row.Status] ?? row.Status}
                      </span>
                      {row.Reason && (
                        <span className="text-muted-foreground">
                          ({REASON_LABEL[row.Reason] ?? row.Reason})
                        </span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </DialogBody>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isImporting}>
            {result ? "关闭" : "取消"}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!text.trim() || isImporting}>
            {isImporting ? "导入中…" : "添加"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
