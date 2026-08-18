import type { ModelTemplate } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Plus, X } from "lucide-react";

type TemplateEditorDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedModelId: number | null;
  loading: boolean;
  templateData: ModelTemplate | null;
  newItem: string;
  onNewItemChange: (value: string) => void;
  onAdd: () => void;
  onDelete: (name: string) => void;
};

export function TemplateEditorDialog({
  open,
  onOpenChange,
  selectedModelId,
  loading,
  templateData,
  newItem,
  onNewItemChange,
  onAdd,
  onDelete,
}: TemplateEditorDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>模板编辑</DialogTitle>
          <DialogDescription>
            模板用于自动关联匹配：区分大小写，自动包含 Model.Name 与既有关联 ProviderModel；此处可手动补充别名。
          </DialogDescription>
        </DialogHeader>

        {!selectedModelId ? (
          <div className="text-sm text-muted-foreground">请先选择一个模型</div>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="flex items-center justify-between gap-3">
              <div className="text-xs text-muted-foreground">
                当前模型：<span className="font-mono">{selectedModelId}</span>
              </div>
              {loading && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Spinner className="h-4 w-4" />
                  加载中...
                </div>
              )}
            </div>

            <div className="flex flex-col sm:flex-row gap-2">
              <Input
                placeholder="新增模板项（区分大小写，如 gpt-4o-2024-08-06）"
                value={newItem}
                onChange={(event) => onNewItemChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    onAdd();
                  }
                }}
                className="h-8 text-xs"
                disabled={loading}
              />
              <Button onClick={onAdd} className="h-8 text-xs sm:w-auto" disabled={loading || newItem.trim().length === 0}>
                <Plus className="h-4 w-4 mr-1" />
                添加
              </Button>
            </div>

            {(templateData?.items ?? []).length === 0 ? (
              <div className="text-xs text-muted-foreground">暂无模板项</div>
            ) : (
              <div className="min-h-0 flex-1 overflow-auto rounded-md border">
                <Table>
                  <TableHeader className="sticky top-0 bg-background">
                    <TableRow>
                      <TableHead className="w-[55%]">模板项</TableHead>
                      <TableHead className="w-[35%]">来源</TableHead>
                      <TableHead className="w-[10%] text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {templateData?.items.map((item) => {
                      const canRemoveManual = item.sources.includes("manual");
                      return (
                        <TableRow key={item.name}>
                          <TableCell className="py-2">
                            <span className="font-mono text-xs break-all">{item.name}</span>
                          </TableCell>
                          <TableCell className="py-2">
                            <span className="text-xs text-muted-foreground">{item.sources.join(", ")}</span>
                          </TableCell>
                          <TableCell className="py-2 text-right">
                            {canRemoveManual && (
                              <Button
                                variant="ghost"
                                className="h-7 w-7 p-0"
                                onClick={() => onDelete(item.name)}
                                disabled={loading}
                                title="删除手动模板项"
                              >
                                <X className="h-4 w-4" />
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

