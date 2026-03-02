import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import type { Provider } from "@/lib/api";

interface BlacklistManagementDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  blacklistedProviders: Provider[];
  onOpenProviderSelector: () => void;
  onRemoveProvider: (providerId: number) => void;
}

export function BlacklistManagementDialog({
  open,
  onOpenChange,
  blacklistedProviders,
  onOpenProviderSelector,
  onRemoveProvider,
}: BlacklistManagementDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[560px] max-h-[80vh] max-w-3xl flex-col">
        <DialogHeader>
          <DialogTitle>拉黑管理</DialogTitle>
          <DialogDescription>管理已拉黑的提供商，拉黑后虚拟模型不会请求该提供商的模型</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col space-y-4">
          <div className="flex items-center justify-between">
            <h4 className="font-medium">已拉黑提供商</h4>
            <Button onClick={onOpenProviderSelector}>添加</Button>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {blacklistedProviders.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={3} className="text-center text-muted-foreground">
                      暂无拉黑提供商
                    </TableCell>
                  </TableRow>
                ) : (
                  blacklistedProviders.map((provider) => (
                    <TableRow key={provider.ID}>
                      <TableCell>{provider.Name}</TableCell>
                      <TableCell>{provider.Type}</TableCell>
                      <TableCell>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => onRemoveProvider(provider.ID)}
                        >
                          解除拉黑
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
