import { useState } from "react";
import { Plus } from "lucide-react";
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
import type { Provider } from "@/lib/api";

interface AllModelsCustomAddProps {
  value: string;
  onChange: (value: string) => void;
  /** 返回 true 表示写入成功（成功路径会清空输入），据此决定二级弹窗是否关闭 */
  onAdd: () => Promise<boolean>;
  onClose: () => void;
  addingModels: boolean;
  batchTesting: boolean;
  provider: Provider | null;
}

/** 底部只留触发按钮，输入收进固定尺寸的二级弹窗：常驻 textarea 换行/长名字会让
 *  主弹窗底部区域膨胀，挤占模型列表可视高度。 */
export function AllModelsCustomAdd({
  value,
  onChange,
  onAdd,
  onClose,
  addingModels,
  batchTesting,
  provider,
}: AllModelsCustomAddProps) {
  const [open, setOpen] = useState(false);

  const handleSubmit = async () => {
    const added = await onAdd();
    if (added) {
      setOpen(false);
    }
  };

  return (
    <div className="flex items-center gap-3 flex-shrink-0">
      <Button
        size="sm"
        variant="outline"
        onClick={() => setOpen(true)}
        disabled={!provider || batchTesting}
      >
        <Plus className="h-4 w-4" />
        添加模型
      </Button>
      <span className="text-xs text-muted-foreground">
        自定义模型或补充上游未返回的模型
      </span>
      <Button
        variant="outline"
        size="sm"
        className="ml-auto"
        onClick={onClose}
      >
        关闭
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent size="sm">
          <DialogHeader>
            <DialogTitle>添加自定义模型</DialogTitle>
            <DialogDescription>
              每行一个模型 ID，可来自定义或补充上游未返回的模型。
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Textarea
              value={value}
              onChange={(e) => onChange(e.target.value)}
              placeholder="每行一个模型 ID"
              className="h-40 resize-none"
              autoFocus
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={addingModels || batchTesting}
            >
              {addingModels ? "提交中..." : "添加"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
