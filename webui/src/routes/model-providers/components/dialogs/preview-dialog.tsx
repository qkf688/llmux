import type { AssociationPreview } from "@/lib/api";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type PreviewDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  type: "associate" | "clean";
  data: AssociationPreview[];
  executing: boolean;
  onConfirm: () => void;
};

export function PreviewDialog({
  open,
  onOpenChange,
  type,
  data,
  executing,
  onConfirm,
}: PreviewDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>
            {type === "associate" ? "一键关联预览" : "清除无效关联预览"}
          </DialogTitle>
          <DialogDescription>
            {type === "associate"
              ? `将添加 ${data.length} 个新关联`
              : `将删除 ${data.length} 个无效关联`}
          </DialogDescription>
        </DialogHeader>

        <DialogBody className="rounded-md border">
          {data.length === 0 ? (
            <div className="flex items-center justify-center h-32 text-muted-foreground">
              {type === "associate" ? "没有可添加的关联" : "没有无效关联"}
            </div>
          ) : (
            <Table>
              <TableHeader className="sticky top-0 bg-secondary/80">
                <TableRow>
                  <TableHead>模型</TableHead>
                  <TableHead>提供商</TableHead>
                  <TableHead>提供商模型</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.map((item, index) => (
                  <TableRow key={index}>
                    <TableCell className="font-medium">{item.model_name}</TableCell>
                    <TableCell>{item.provider_name}</TableCell>
                    <TableCell className="text-muted-foreground">{item.provider_model}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </DialogBody>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={executing}>
            取消
          </Button>
          <Button onClick={onConfirm} disabled={executing || data.length === 0}>
            {executing ? "执行中..." : "确认执行"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

