import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { Provider } from "@/lib/api";

interface AllModelsCustomAddProps {
  value: string;
  onChange: (value: string) => void;
  onAdd: () => void;
  onClose: () => void;
  addingModels: boolean;
  batchTesting: boolean;
  provider: Provider | null;
}

export function AllModelsCustomAdd({
  value,
  onChange,
  onAdd,
  onClose,
  addingModels,
  batchTesting,
  provider,
}: AllModelsCustomAddProps) {
  return (
    <div className="flex items-center justify-between gap-3 flex-shrink-0">
      <Textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="每行一个模型 ID，可用来自定义或补充上游未返回的模型"
        className="h-16 resize-none flex-1"
      />
      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={onAdd}
          disabled={addingModels || !provider || batchTesting}
        >
          {addingModels ? "提交中..." : "添加"}
        </Button>
        <Button variant="outline" size="sm" onClick={onClose}>
          关闭
        </Button>
      </div>
    </div>
  );
}
