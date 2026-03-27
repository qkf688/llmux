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
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";
import { extractAllModels } from "../../utils/config";

interface ProvidersMobileListProps {
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

export function ProvidersMobileList({
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
}: ProvidersMobileListProps) {
  return (
    <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2 divide-y divide-border">
      {providers.map((provider) => {
        const allModels = extractAllModels(provider.Config);
        return (
          <div key={provider.ID} className="py-2 space-y-2">
            <div className="min-w-0">
              <h3 className="font-semibold text-[13px] leading-snug whitespace-normal break-all">{provider.Name}</h3>
              <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 pt-0.5 text-[10px] text-muted-foreground leading-tight">
                <span className="shrink-0">ID: {provider.ID}</span>
                <span className="shrink-0">类型: {provider.Type || "未知"}</span>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <Button
                variant="outline"
                size="sm"
                className="h-6 px-2 text-[11px] gap-1.5"
                onClick={() => onOpenAllModelsDialog(provider)}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  className="h-3 w-3"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
                  />
                </svg>
                {allModels.length}
              </Button>

              <div className="flex items-center gap-1.5">
                <span className="text-[10px] text-muted-foreground">端点</span>
                <Switch
                  checked={provider.ModelEndpoint ?? true}
                  onCheckedChange={() => onToggleModelEndpoint(provider)}
                  className="scale-75"
                />
              </div>

              <div className="flex items-center gap-1.5">
                <span className="text-[10px] text-muted-foreground">关联</span>
                <Switch
                  checked={!(provider.blacklisted ?? false)}
                  onCheckedChange={(checked) => onToggleAssociationTrigger(provider, checked)}
                  disabled={updatingAssociationTrigger[provider.ID]}
                  className="scale-75"
                />
              </div>

              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="flex items-center gap-1.5">
                      <span className="text-[10px] text-muted-foreground">过滤</span>
                      <Switch
                        checked={provider.ModelFilterEnabled ?? false}
                        onCheckedChange={(checked) => onToggleModelFilter(provider, checked)}
                        disabled={updatingFilter[provider.ID]}
                        className="scale-75"
                      />
                    </div>
                  </TooltipTrigger>
                  <TooltipContent>启用模型过滤</TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>

            <div className="flex flex-wrap justify-end gap-1.5 pt-0.5">
              <Button
                variant="outline"
                size="sm"
                className="h-6 px-2 text-[11px]"
                onClick={() => onEditProvider(provider)}
              >
                编辑
              </Button>
              <Button
                variant="secondary"
                size="sm"
                className="h-6 px-2 text-[11px]"
                onClick={() => onOpenModelsDialog(provider.ID)}
              >
                模型
              </Button>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-6 px-2 text-[11px]"
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
                  <Button
                    variant="destructive"
                    size="sm"
                    className="h-6 px-2 text-[11px]"
                    onClick={() => onOpenDeleteDialog(provider.ID)}
                  >
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
          </div>
        );
      })}
    </div>
  );
}

