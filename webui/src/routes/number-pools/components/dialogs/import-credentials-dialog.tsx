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

type ImportResult = { imported: number; skipped: number };

type ImportCredentialsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 父组件执行解析 / 去重 / 落库，返回导入与跳过数量 */
  onImport: (text: string) => ImportResult;
};

export function ImportCredentialsDialog({ open, onOpenChange, onImport }: ImportCredentialsDialogProps) {
  const [text, setText] = useState("");
  const [result, setResult] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // 每次打开重置，聚焦输入区
  useEffect(() => {
    if (open) {
      setText("");
      setResult(null);
      textareaRef.current?.focus();
    }
  }, [open]);

  const handleSubmit = () => {
    const { imported, skipped } = onImport(text);
    if (imported === 0 && skipped === 0) return;
    setResult(`导入 ${imported} 条，跳过重复 ${skipped} 条${imported === 0 ? "（全部已存在）" : ""}`);
    setText("");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>添加凭据</DialogTitle>
          <DialogDescription>
            粘贴 Key，每行一个（逗号分隔亦可）；只填 1 个也合法。自动去重，正式实现（S1 起）加密落库。
          </DialogDescription>
        </DialogHeader>

        <DialogBody className="-mx-1 min-w-0 px-1">
          <Textarea
            ref={textareaRef}
            rows={8}
            className="font-mono text-xs"
            placeholder={"sk-...\nsk-...\nsk-..."}
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          {result && <p className="mt-2 text-xs text-muted-foreground">{result}</p>}
        </DialogBody>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={!text.trim()}>
            添加
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}