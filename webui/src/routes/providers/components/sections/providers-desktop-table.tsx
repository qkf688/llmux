import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";
import { extractAllModels } from "../../utils/config";

interface ProvidersDesktopTableProps {
  providers: Provider[];
  updatingFilter: Record<number, boolean>;
  updatingAssociationTrigger: Record<number, boolean>;
  clearingAssociation: boolean;

  onOpenAllModelsDialog: (provider: Provider) => void | Promise<void>;
  onToggleModelEndpoint: (provider: Provider) => void | Promise<void>;
  onToggleAssociationTrigger: (provider: Provider, checked: boolean) => void | Promise<void>;
  onToggleModelFilter: (provider: Provider, checked: boolean) => void | Promise<void>;
  onEditProvider: (provider: Provider) => void;
  onOpenModelsDialog: (providerId: number) => void | Promise<void>;

  onOpenClearAssociationsDialog: (providerId: number) => void;
  onCancelClearAssociationsDialog: () => void;
  onHandleClearAssociations: () => void | Promise<void>;

  onOpenDeleteDialog: (providerId: number) => void;
  onCancelDeleteDialog: () => void;
  onHandleDelete: () => void | Promise<void>;
}

export function ProvidersDesktopTable({
  providers,
  updatingFilter,
  updatingAssociationTrigger,
  clearingAssociation,
  onOpenAllModelsDialog,
  onToggleModelEndpoint,
  onToggleAssociationTrigger,
  onToggleModelFilter,
  onEditProvider,
  onOpenModelsDialog,
  onOpenClearAssociationsDialog,
  onCancelClearAssociationsDialog,
  onHandleClearAssociations,
  onOpenDeleteDialog,
  onCancelDeleteDialog,
  onHandleDelete,
}: ProvidersDesktopTableProps) {
  return (
    <div className="hidden sm:block w-full overflow-x-auto">
      <Table className="min-w-[1200px]">
        <TableHeader className="z-10 sticky top-0 bg-secondary/80 text-secondary-foreground">
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>名称</TableHead>
            <TableHead>类型</TableHead>
            <TableHead>全部模型</TableHead>
            <TableHead>模型端点</TableHead>
            <TableHead>关联触发</TableHead>
            <TableHead className="w-[360px]">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {providers.map((provider) => {
            const allModels = extractAllModels(provider.Config);
            return (
              <TableRow key={provider.ID}>
                <TableCell className="font-mono text-xs text-muted-foreground">{provider.ID}</TableCell>
                <TableCell className="font-medium">{provider.Name}</TableCell>
                <TableCell className="text-sm">{provider.Type}</TableCell>
                <TableCell>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => onOpenAllModelsDialog(provider)}
                    className="gap-1.5"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      className="h-4 w-4"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
                      />
                    </svg>
                    {allModels.length}
                  </Button>
                </TableCell>
                <TableCell>
                  <Switch
                    checked={provider.ModelEndpoint ?? true}
                    onCheckedChange={() => onToggleModelEndpoint(provider)}
                  />
                </TableCell>
                <TableCell>
                  <Switch
                    checked={!(provider.blacklisted ?? false)}
                    onCheckedChange={(checked) => onToggleAssociationTrigger(provider, checked)}
                    disabled={updatingAssociationTrigger[provider.ID]}
                  />
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-2 items-center">
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <div className="flex items-center">
                            <Switch
                              checked={provider.ModelFilterEnabled ?? false}
                              onCheckedChange={(checked) => onToggleModelFilter(provider, checked)}
                              disabled={updatingFilter[provider.ID]}
                            />
                          </div>
                        </TooltipTrigger>
                        <TooltipContent>启用模型过滤</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                    <Button variant="outline" size="sm" onClick={() => onEditProvider(provider)}>
                      编辑
                    </Button>
                    <Button variant="secondary" size="sm" onClick={() => onOpenModelsDialog(provider.ID)}>
                      获取模型
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => onOpenClearAssociationsDialog(provider.ID)}
                        >
                          清除关联
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>确定要清除这个提供商的所有关联吗？</AlertDialogTitle>
                          <AlertDialogDescription>
                            此操作将删除该提供商下所有的模型关联关系，但不会删除提供商本身。此操作无法撤销。
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel onClick={onCancelClearAssociationsDialog}>取消</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={onHandleClearAssociations}
                            disabled={clearingAssociation}
                            className="bg-destructive hover:bg-destructive/90"
                          >
                            {clearingAssociation ? "清除中..." : "确认清除"}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" size="sm" onClick={() => onOpenDeleteDialog(provider.ID)}>
                          删除
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>确定要删除这个提供商吗？</AlertDialogTitle>
                          <AlertDialogDescription>此操作无法撤销。这将永久删除该提供商。</AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel onClick={onCancelDeleteDialog}>取消</AlertDialogCancel>
                          <AlertDialogAction onClick={onHandleDelete}>确认删除</AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

