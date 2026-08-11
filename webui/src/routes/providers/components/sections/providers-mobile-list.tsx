import { Boxes } from "lucide-react";
import { AnimatePresence } from "motion/react";
import { AnimatedListItem } from "@/components/ui/animated-list-item";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";
import { extractAllModels } from "../../utils/config";
import { ProviderRowActions } from "./provider-row-actions";

// 卡片副标题（ID / 类型）行样式：单独成常量避免 JSX 内长行
const META_ROW_CLS =
  "flex flex-wrap items-center gap-x-2 gap-y-0.5 pt-0.5 text-[10px] text-muted-foreground leading-tight";

interface ProvidersMobileListProps {
  providers: Provider[];
  updatingFilter: Record<number, boolean>;
  updatingAssociationTrigger: Record<number, boolean>;
  clearingAssociation: boolean;
  deleting: boolean;

  onOpenAllModelsDialog: (provider: Provider) => void | Promise<void>;
  onToggleModelEndpoint: (provider: Provider) => void | Promise<void>;
  onToggleAssociationTrigger: (
    provider: Provider,
    checked: boolean,
  ) => void | Promise<void>;
  onToggleModelFilter: (
    provider: Provider,
    checked: boolean,
  ) => void | Promise<void>;
  onEditProvider: (provider: Provider) => void;
  onOpenModelsDialog: (providerId: number) => void | Promise<void>;

  onOpenClearAssociationsDialog: (providerId: number) => void;
  onCancelClearAssociationsDialog: () => void;
  onHandleClearAssociations: (providerId: number) => void | Promise<void>;

  onOpenDeleteDialog: (providerId: number) => void;
  onCancelDeleteDialog: () => void;
  onHandleDelete: (providerId: number) => void | Promise<void>;
}

export function ProvidersMobileList({
  providers,
  updatingFilter,
  updatingAssociationTrigger,
  clearingAssociation,
  deleting,
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
    // 分隔线挂在每张卡片上（而非容器 divide-y）：退场动画会把卡片高度收起，
    // 容器级 divide-y 会在收起过程中留下一条无主的残线
    <div className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2">
      <AnimatePresence initial={false}>
        {providers.map((provider) => {
          const allModels = extractAllModels(provider.Config);
          return (
            <AnimatedListItem
              key={provider.ID}
              className="py-2 space-y-2 border-b last:border-b-0"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <h3 className="font-semibold text-[13px] leading-snug whitespace-normal break-all">
                    {provider.Name}
                  </h3>
                  <div className={META_ROW_CLS}>
                    <span className="shrink-0">ID: {provider.ID}</span>
                    <span className="shrink-0">
                      类型: {provider.Type || "未知"}
                    </span>
                  </div>
                </div>
                {/* 编辑 + ⋯ 菜单，和桌面端复用同一入口，避免两套动作按钮 */}
                <ProviderRowActions
                  provider={provider}
                  clearingAssociation={clearingAssociation}
                  deleting={deleting}
                  compact
                  onEditProvider={onEditProvider}
                  onOpenModelsDialog={onOpenModelsDialog}
                  onOpenClearAssociationsDialog={onOpenClearAssociationsDialog}
                  onCancelClearAssociationsDialog={
                    onCancelClearAssociationsDialog
                  }
                  onHandleClearAssociations={onHandleClearAssociations}
                  onOpenDeleteDialog={onOpenDeleteDialog}
                  onCancelDeleteDialog={onCancelDeleteDialog}
                  onHandleDelete={onHandleDelete}
                />
              </div>

              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <Button
                  variant="outline"
                  size="sm"
                  className="h-6 px-2 text-[11px] gap-1.5"
                  onClick={() => onOpenAllModelsDialog(provider)}
                >
                  <Boxes className="size-3 opacity-70" />
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
                    onCheckedChange={(checked) =>
                      onToggleAssociationTrigger(provider, checked)
                    }
                    disabled={updatingAssociationTrigger[provider.ID]}
                    className="scale-75"
                  />
                </div>

                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="flex items-center gap-1.5">
                      <span className="text-[10px] text-muted-foreground">
                        过滤
                      </span>
                      <Switch
                        checked={provider.ModelFilterEnabled ?? false}
                        onCheckedChange={(checked) =>
                          onToggleModelFilter(provider, checked)
                        }
                        disabled={updatingFilter[provider.ID]}
                        className="scale-75"
                      />
                    </div>
                  </TooltipTrigger>
                  <TooltipContent>启用模型过滤</TooltipContent>
                </Tooltip>
              </div>
            </AnimatedListItem>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
