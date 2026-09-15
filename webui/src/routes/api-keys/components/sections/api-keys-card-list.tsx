/**
 * API Keys 卡片列表（octopus 风格）：一 key 一卡，**行内不渲染 key 明文/掩码**——
 * 行上的明文出口只有「复制」按钮（进剪贴板）。此前表格 + 眼睛方案在展开明文时
 * 会把表格向右撑开（用户 2026-09-16 反馈）；查看明文改由编辑弹窗承载（定宽
 * 布局无撑开问题，且默认掩码 + 显式点击，IP 环境复制失败时的手动兜底在那儿）。
 *
 * 行右侧动作与 octopus 对齐：编辑（改名 + 查看/复制）、复制明文、删除确认；
 * 启停 Switch 保留在行内（octopus 把 enabled 收进编辑表单，本页维持
 * 一键启停的既有习惯）。后端解密失败降级行（api_key = "****"）无明文可复制，
 * 复制按钮禁用并给出警示态（沿 octopus legacy-hashed 的处理姿势）。
 */
import { AlertTriangle, Copy, KeyRound, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { StaggerItem, StaggerList } from "@/components/ui/stagger-list";
import { copySecretWithToast } from "@/lib/clipboard";
import type { ApiKeyItem } from "@/lib/api";
import { formatDateTime } from "@/lib/formatters";
import { cn } from "@/lib/utils";

/** handler/apikeys apiKeyResponse 解密失败降级值（与 credentialcrypto.MaskStored 同口径） */
const MASKED_FALLBACK = "****";

// 复制失败文案指向编辑弹窗：本页行内永不渲染明文，唯一的手动复制路径在那儿
const COPY_FAILURE_HINT =
  "复制失败：浏览器限制了自动复制。请在该 key 的编辑弹窗点「显示明文」后手动选中复制。";

type ApiKeysCardListProps = {
  apiKeys: ApiKeyItem[];
  onToggle: (apiKey: ApiKeyItem) => void;
  onEdit: (apiKey: ApiKeyItem) => void;
  /** 请求删除（弹确认框），不直接删除 */
  onRequestDelete: (apiKey: ApiKeyItem) => void;
  /** 任一 mutation 在途时禁用启停开关，防 PATCH 视觉未动期间的连点 */
  busy?: boolean;
};

export function ApiKeysCardList({
  apiKeys,
  onToggle,
  onEdit,
  onRequestDelete,
  busy = false,
}: ApiKeysCardListProps) {
  return (
    <StaggerList className="min-h-0 flex-1 space-y-3 overflow-y-auto pb-2">
      {apiKeys.map((apiKey) => (
        <StaggerItem
          key={apiKey.id}
          className={cn(
            "flex items-center justify-between gap-3 rounded-xl border bg-card p-4 shadow-sm transition-colors",
            !apiKey.enabled && "border-border/60 bg-muted/30",
          )}
        >
          <div className="flex min-w-0 items-center gap-3">
            <span
              className={cn(
                "grid size-9 shrink-0 place-items-center rounded-lg",
                apiKey.enabled
                  ? "bg-primary/10 text-primary"
                  : "bg-muted text-muted-foreground",
              )}
            >
              <KeyRound className="size-4" />
            </span>
            {/* 密钥值不进 DOM：name 与元信息均为定宽截断，行宽与内容长度解耦 */}
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold leading-tight">
                {apiKey.name}
              </div>
              <div className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                <span
                  className={cn(
                    "inline-flex items-center gap-1",
                    apiKey.enabled && "text-emerald-600 dark:text-emerald-400",
                  )}
                >
                  <span
                    className={cn(
                      "size-1.5 rounded-full",
                      apiKey.enabled
                        ? "bg-emerald-500"
                        : "bg-muted-foreground/50",
                    )}
                  />
                  {apiKey.enabled ? "已启用" : "已停用"}
                </span>
                <span>· 创建于 {formatDateTime(apiKey.created_at)}</span>
              </div>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-1">
            <Switch
              checked={apiKey.enabled}
              disabled={busy}
              onCheckedChange={() => onToggle(apiKey)}
              aria-label={`${apiKey.enabled ? "停用" : "启用"} ${apiKey.name}`}
              className="mr-2"
            />
            {/* 按钮语义由 aria-label 承载，不设 title（沿号池密集操作区的决定） */}
            <Button
              variant="ghost"
              size="icon"
              className="size-8 text-muted-foreground hover:bg-muted hover:text-foreground"
              onClick={() => onEdit(apiKey)}
              aria-label={`编辑 ${apiKey.name}`}
            >
              <Pencil className="size-4" />
            </Button>
            {apiKey.api_key === MASKED_FALLBACK ? (
              <Button
                variant="ghost"
                size="icon"
                disabled
                className="size-8 bg-amber-500/10 text-amber-600 dark:text-amber-400"
                aria-label={`密钥 ${apiKey.name} 明文不可用，无法复制`}
              >
                <AlertTriangle className="size-4" />
              </Button>
            ) : (
              <Button
                variant="ghost"
                size="icon"
                className="size-8 bg-primary/10 text-primary transition-all hover:bg-primary hover:text-primary-foreground active:scale-95"
                onClick={() =>
                  void copySecretWithToast(
                    apiKey.api_key,
                    `已复制密钥 ${apiKey.name}`,
                    COPY_FAILURE_HINT,
                  )
                }
                aria-label={`复制密钥 ${apiKey.name}`}
              >
                <Copy className="size-4" />
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon"
              className="size-8 bg-destructive/10 text-destructive hover:bg-destructive hover:text-destructive-foreground"
              onClick={() => onRequestDelete(apiKey)}
              aria-label={`删除 ${apiKey.name}`}
            >
              <Trash2 className="size-4" />
            </Button>
          </div>
        </StaggerItem>
      ))}
    </StaggerList>
  );
}
