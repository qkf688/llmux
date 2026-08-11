import { useState } from "react";
import { MoreHorizontal, Pencil, RefreshCw, Trash2, Unlink } from "lucide-react";
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
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Provider } from "@/lib/api";

interface ProviderRowActionsProps {
  provider: Provider;
  /** 移动端用更小的控件尺寸；桌面端保持默认 */
  compact?: boolean;

  onEditProvider: (provider: Provider) => void;
  onOpenModelsDialog: (providerId: number) => void | Promise<void>;

  onHandleClearAssociations: (providerId: number) => void | Promise<void>;
  onHandleDelete: (providerId: number) => void | Promise<void>;
}

/**
 * 单行的操作入口：行内只留「编辑」，低频动作收进 ⋯ 菜单。
 *
 * 确认弹窗必须受控且渲染在 DropdownMenu 之外——菜单项被选中后 Radix 会卸载
 * 菜单内容，若把 AlertDialog 嵌在菜单里会随之卸载，弹窗永远来不及出现。
 *
 * 弹窗 open 由本组件本地 state（deleteOpen/clearOpen）控制；确认 handler
 * 接收 providerId 参数，防重复提交由 hook 内 guard 负责。
 */
export function ProviderRowActions({
  provider,
  compact = false,
  onEditProvider,
  onOpenModelsDialog,
  onHandleClearAssociations,
  onHandleDelete,
}: ProviderRowActionsProps) {
  const [clearOpen, setClearOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const triggerSize = compact ? "size-6" : "size-8";
  const iconSize = compact ? "size-3" : "size-4";

  return (
    <>
      <div className="flex items-center gap-1.5">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              className={triggerSize}
              onClick={() => onEditProvider(provider)}
            >
              <Pencil className={iconSize} />
              <span className="sr-only">编辑</span>
            </Button>
          </TooltipTrigger>
          <TooltipContent>编辑</TooltipContent>
        </Tooltip>

        {/* modal={false}：菜单不锁 body 的 pointer-events。
            否则从菜单项打开页面顶层的 dialog（如「获取模型」）时，菜单关闭与
            dialog 的 body 锁交错，会残留 pointer-events:none 导致整页点不动 */}
        <DropdownMenu modal={false}>
          <Tooltip>
            <TooltipTrigger asChild>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon" className={triggerSize}>
                  <MoreHorizontal className={iconSize} />
                  <span className="sr-only">更多操作</span>
                </Button>
              </DropdownMenuTrigger>
            </TooltipTrigger>
            <TooltipContent>更多操作</TooltipContent>
          </Tooltip>
          {/* 菜单关闭时不要把焦点抢回触发器，否则会和随后打开的确认弹窗争焦点 */}
          <DropdownMenuContent
            align="end"
            className="min-w-44"
            onCloseAutoFocus={(e) => e.preventDefault()}
          >
            <DropdownMenuItem onSelect={() => onOpenModelsDialog(provider.ID)}>
              <RefreshCw />
              获取模型
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setClearOpen(true)}>
              <Unlink />
              清除关联
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => setDeleteOpen(true)}>
              <Trash2 />
              删除提供商
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <AlertDialog open={clearOpen} onOpenChange={setClearOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              确定要清除这个提供商的所有关联吗？
            </AlertDialogTitle>
            <AlertDialogDescription>
              此操作将删除该提供商下所有的模型关联关系，但不会删除提供商本身。此操作无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            {/* 点击后 Radix 立即关闭弹窗，首次点击看不到 disabled/loading 态：
                loading 反馈交给 toast，防重复提交由 hook 内 guard 负责 */}
            <AlertDialogAction
              onClick={() => onHandleClearAssociations(provider.ID)}
              className="bg-destructive hover:bg-destructive/90"
            >
              确认清除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确定要删除这个提供商吗？</AlertDialogTitle>
            <AlertDialogDescription>
              此操作无法撤销。这将永久删除该提供商。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            {/* 同上：loading 反馈交给 toast，防重复提交由 hook 内 guard 负责 */}
            <AlertDialogAction onClick={() => onHandleDelete(provider.ID)}>
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
