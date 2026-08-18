import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import {
  FileText,
  Globe,
  ListOrdered,
  Scale,
  SlidersHorizontal,
  TriangleAlert,
  X,
  CheckCircle,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type ActionMenuProps = {
  operationScope: "current" | "all";
  onOperationScopeChange: (value: "current" | "all") => void;
  selectedModelName: string;
  selectedModelId: number | null;
  modelCount: number;
  resettingWeights: boolean;
  resettingPriorities: boolean;
  enablingAssociations: boolean;
  onResetWeights: () => void;
  onResetPriorities: () => void;
  onEnableAssociations: () => void;
  onToggleTemplateEditor: () => void;
};

type ActionMenuDialogProps = ActionMenuProps & {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function ActionMenuDialog({
  open,
  onOpenChange,
  operationScope,
  onOperationScopeChange,
  selectedModelName,
  selectedModelId,
  modelCount,
  resettingWeights,
  resettingPriorities,
  enablingAssociations,
  onResetWeights,
  onResetPriorities,
  onEnableAssociations,
  onToggleTemplateEditor,
}: ActionMenuDialogProps) {
  const isGlobalScope = operationScope === "all";
  const disableGlobalActions = !selectedModelId && !isGlobalScope;

  const close = () => onOpenChange(false);

  const handleAction = (fn: () => void) => {
    fn();
    close();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="menu" className="rounded-[10px]" showCloseButton={false}>
        <div className="flex min-h-0 flex-1 flex-col">
          <DialogHeader className="px-3.5 py-2.5 border-b">
            <div className="flex items-center justify-between">
              <DialogTitle className="text-sm font-semibold">操作</DialogTitle>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={close}
                aria-label="关闭"
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
            <DialogDescription className="sr-only">操作菜单</DialogDescription>
          </DialogHeader>

          <DialogBody className="py-1">
            {/* 作用域切换（segmented control，高对比选中态） */}
            <div className="mx-2.5 mt-2 mb-1 p-2 bg-secondary rounded-lg">
              <div className="text-[10px] text-muted-foreground uppercase tracking-wide mb-1.5">作用域</div>
              <div className="grid grid-cols-2 gap-1 bg-muted rounded-md p-0.5">
                <button
                  onClick={() => onOperationScopeChange("current")}
                  className={`flex items-center justify-center gap-1.5 px-2 py-1.5 text-xs rounded-[5px] transition-all whitespace-nowrap ${
                    !isGlobalScope
                      ? "bg-primary text-primary-foreground font-medium shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <SlidersHorizontal className="h-3 w-3 flex-shrink-0" />
                  <span className="truncate max-w-[90px]">{selectedModelName}</span>
                </button>
                <button
                  onClick={() => onOperationScopeChange("all")}
                  className={`flex items-center justify-center gap-1.5 px-2 py-1.5 text-xs rounded-[5px] transition-all whitespace-nowrap ${
                    isGlobalScope
                      ? "bg-warning text-warning-foreground font-medium shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <Globe className="h-3 w-3 flex-shrink-0" />
                  全部模型
                </button>
              </div>
            </div>

            {/* 作用域影响提示（两种作用域都显示，保持高度稳定） */}
            <div
              className={`mx-2.5 mb-1 px-2.5 py-1.5 rounded-md text-[11px] flex items-center gap-1.5 transition-colors ${
                isGlobalScope
                  ? "bg-warning-tint text-warning-foreground"
                  : "bg-secondary text-muted-foreground"
              }`}
            >
              {isGlobalScope ? (
                <TriangleAlert className="h-3 w-3 flex-shrink-0" />
              ) : (
                <SlidersHorizontal className="h-3 w-3 flex-shrink-0" />
              )}
              <span className="truncate">
                {isGlobalScope
                  ? `"随作用域"操作将影响全部 ${modelCount} 个模型`
                  : `"随作用域"操作将作用于：${selectedModelName}`}
              </span>
            </div>

            {/* ── 关联状态（随作用域） ── */}
            <SectionLabel badge="scope">关联状态</SectionLabel>
            <MenuItem
              icon={CheckCircle}
              scoped
              globalActive={isGlobalScope}
              disabled={disableGlobalActions || enablingAssociations}
              onClick={() => handleAction(onEnableAssociations)}
              title="启用所有关联"
              desc="将关联置为启用"
              loading={enablingAssociations}
            />

            <Divider />

            {/* ── 权重与优先级（随作用域） ── */}
            <SectionLabel badge="scope">权重与优先级</SectionLabel>
            <MenuItem
              icon={Scale}
              scoped
              globalActive={isGlobalScope}
              warning={isGlobalScope}
              disabled={disableGlobalActions || resettingWeights}
              onClick={() => handleAction(onResetWeights)}
              title="重置权重"
              desc="恢复到默认权重"
              loading={resettingWeights}
            />
            <MenuItem
              icon={ListOrdered}
              scoped
              globalActive={isGlobalScope}
              warning={isGlobalScope}
              disabled={disableGlobalActions || resettingPriorities}
              onClick={() => handleAction(onResetPriorities)}
              title="重置优先级"
              desc="恢复到默认优先级"
              loading={resettingPriorities}
            />

            <Divider />

            {/* ── 模板管理（当前模型专属） ── */}
            <SectionLabel badge="scope">模板管理</SectionLabel>
            <MenuItem
              icon={FileText}
              scoped
              disabled={!selectedModelId || isGlobalScope}
              onClick={() => handleAction(onToggleTemplateEditor)}
              title="模板编辑"
              desc="管理模型名映射模板"
            />
          </DialogBody>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function SectionLabel({
  children,
  badge,
}: {
  children: React.ReactNode;
  badge?: "scope";
}) {
  return (
    <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground uppercase tracking-wide px-3.5 pt-1.5 pb-0.5">
      {children}
      {badge === "scope" && (
        <span className="text-[9px] px-1.5 py-px rounded-full bg-info-tint text-info font-medium normal-case tracking-normal">
          随作用域
        </span>
      )}
    </div>
  );
}

function Divider() {
  return <div className="h-px bg-border my-0.5" />;
}

function MenuItem({
  icon: Icon,
  scoped,
  globalActive,
  warning,
  danger,
  disabled,
  loading,
  onClick,
  title,
  desc,
}: {
  icon: LucideIcon;
  scoped?: boolean;
  globalActive?: boolean;
  warning?: boolean;
  danger?: boolean;
  disabled?: boolean;
  loading?: boolean;
  onClick: () => void;
  title: string;
  desc: string;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`flex items-start gap-2 w-full text-left ${
        scoped ? "pl-[18px]" : "pl-3.5"
      } pr-3.5 py-1.5 text-xs disabled:opacity-45 disabled:cursor-not-allowed hover:bg-secondary/50 transition-colors relative ${
        warning ? "text-warning-foreground" : danger ? "text-destructive" : ""
      }`}
    >
      {/* 作用域影响指示条 */}
      {scoped && (
        <span
          className={`absolute left-2.5 top-2 bottom-2 w-0.5 rounded-full ${
            globalActive ? "bg-warning/70" : "bg-info/50"
          }`}
        />
      )}
      {loading ? (
        <Spinner className="h-3 w-3 mt-px flex-shrink-0" />
      ) : (
        <Icon
          className={`h-3 w-3 mt-px flex-shrink-0 ${
            warning ? "text-warning-foreground" : danger ? "text-destructive" : "text-muted-foreground"
          }`}
        />
      )}
      <span className="flex flex-col gap-px flex-1 min-w-0">
        <span className="leading-tight">{title}</span>
        <span className="text-[10px] text-muted-foreground leading-tight">{desc}</span>
      </span>
    </button>
  );
}
