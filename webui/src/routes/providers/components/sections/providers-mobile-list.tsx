import { Boxes } from "lucide-react";
import { AnimatePresence } from "motion/react";
import { AnimatedListItem } from "@/components/ui/animated-list-item";
import { StaggerList } from "@/components/ui/stagger-list";
import { Badge } from "@/components/ui/badge";
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

  onHandleClearAssociations: (providerId: number) => void | Promise<void>;
  onHandleDelete: (providerId: number) => void | Promise<void>;
}

export function ProvidersMobileList({
  providers,
  updatingFilter,
  updatingAssociationTrigger,
  onOpenAllModelsDialog,
  onToggleModelEndpoint,
  onToggleAssociationTrigger,
  onToggleModelFilter,
  onEditProvider,
  onOpenModelsDialog,
  onHandleClearAssociations,
  onHandleDelete,
}: ProvidersMobileListProps) {
  return (
    // 分隔线挂在每张卡片上（而非容器 divide-y）：退场动画会把卡片高度收起，
    // 容器级 divide-y 会在收起过程中留下一条无主的残线。
    // 外层 StaggerList 编排首屏错峰入场（子项传 staggered 才会参与编排）；AnimatePresence 不设
    // initial={false}，首屏交由 stagger 编排，运行时增删仍逐项进出场。不传 resetKey：否则每次增删都整体重播。
    <StaggerList className="sm:hidden flex-1 min-h-0 overflow-y-auto px-2 py-2">
      <AnimatePresence>
        {providers.map((provider) => {
          const allModels = extractAllModels(provider.Config);
          // 端点/分组计数读列表 API 附带值（后端批量 COUNT），不再解析 Config._schedule（S6 已废弃该键）
          const endpointCount = provider.EndpointCount ?? 0;
          const groupCount = provider.GroupCount ?? 0;
          return (
            <AnimatedListItem
              key={provider.ID}
              staggered
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
                  compact
                  onEditProvider={onEditProvider}
                  onOpenModelsDialog={onOpenModelsDialog}
                  onHandleClearAssociations={onHandleClearAssociations}
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

                {endpointCount > 0 || groupCount > 0 ? (
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0.5">
                    端点 {endpointCount} · 分组 {groupCount}
                  </Badge>
                ) : (
                  <span className="text-[10px] text-muted-foreground">未配置调度</span>
                )}

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
    </StaggerList>
  );
}
