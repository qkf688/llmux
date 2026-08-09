import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { Check, X } from "lucide-react";

export type ActionMenuProps = {
  operationScope: "current" | "all";
  onOperationScopeChange: (value: "current" | "all") => void;
  selectedModelName: string;
  selectedModelId: number | null;
  resettingWeights: boolean;
  resettingPriorities: boolean;
  enablingAssociations: boolean;
  onResetWeights: () => void;
  onResetPriorities: () => void;
  onEnableAssociations: () => void;
  onToggleTemplateEditor: () => void;
  onOpenBlacklistDialog: () => void;
  onAutoAssociate: () => void;
  onCleanInvalid: () => void;
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
  resettingWeights,
  resettingPriorities,
  enablingAssociations,
  onResetWeights,
  onResetPriorities,
  onEnableAssociations,
  onToggleTemplateEditor,
  onOpenBlacklistDialog,
  onAutoAssociate,
  onCleanInvalid,
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
      <DialogContent className="sm:max-w-[280px] p-0 gap-0 rounded-[10px]" showCloseButton={false}>
        <div className="flex flex-col max-h-[80vh]">
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

          <div className="flex-1 overflow-y-auto py-1">
            {/* 作用域（切换后不关闭，允许继续执行其他操作） */}
            <SectionLabel>作用域</SectionLabel>
            <MenuItem
              active={!isGlobalScope}
              onClick={() => onOperationScopeChange("current")}
            >
              <span className="truncate flex-1 min-w-0">当前模型：{selectedModelName}</span>
              {!isGlobalScope && <Check className="h-3 w-3 flex-shrink-0" />}
            </MenuItem>
            <MenuItem
              active={isGlobalScope}
              onClick={() => onOperationScopeChange("all")}
            >
              <span className="flex-1">全部模型</span>
              {isGlobalScope && <Check className="h-3 w-3 flex-shrink-0" />}
            </MenuItem>

            <Divider />

            {/* 全局操作 */}
            <SectionLabel>全局操作</SectionLabel>
            <MenuItem
              disabled={disableGlobalActions || enablingAssociations}
              onClick={() => handleAction(onEnableAssociations)}
            >
              {enablingAssociations ? <Spinner className="h-3 w-3 mr-2" /> : null}
              启用所有关联
            </MenuItem>
            <MenuItem
              disabled={disableGlobalActions || resettingWeights}
              onClick={() => handleAction(onResetWeights)}
            >
              {resettingWeights ? <Spinner className="h-3 w-3 mr-2" /> : null}
              重置权重
            </MenuItem>
            <MenuItem
              disabled={disableGlobalActions || resettingPriorities}
              onClick={() => handleAction(onResetPriorities)}
            >
              {resettingPriorities ? <Spinner className="h-3 w-3 mr-2" /> : null}
              重置优先级
            </MenuItem>

            <Divider />

            {/* 编辑管理 */}
            <SectionLabel>编辑管理</SectionLabel>
            <MenuItem disabled={!selectedModelId} onClick={() => handleAction(onToggleTemplateEditor)}>
              模板编辑
            </MenuItem>
            <MenuItem onClick={() => handleAction(onOpenBlacklistDialog)}>
              拉黑管理
            </MenuItem>

            <Divider />

            {/* 关联维护 */}
            <SectionLabel>关联维护</SectionLabel>
            <MenuItem onClick={() => handleAction(onAutoAssociate)}>
              一键关联
            </MenuItem>
            <MenuItem onClick={() => handleAction(onCleanInvalid)}>
              清除无效
            </MenuItem>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="text-[9px] text-muted-foreground uppercase tracking-wide px-3.5 pt-1.5 pb-0.5">
      {children}
    </div>
  );
}

function Divider() {
  return <div className="h-px bg-border my-0.5" />;
}

function MenuItem({
  active,
  disabled,
  onClick,
  children,
}: {
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`flex items-center w-full text-left px-3.5 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed ${
        active ? "bg-secondary" : "hover:bg-secondary/50"
      }`}
    >
      {children}
    </button>
  );
}
