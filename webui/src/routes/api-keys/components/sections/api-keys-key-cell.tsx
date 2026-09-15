/**
 * API Key 行内单元格：默认掩码短形 + 眼睛切换明文 + 复制。
 *
 * 与号池 credential-key-cell 视觉语义一致但**不共用组件**：那边的数据源是
 * 「受控 raw 端点按需取明文」（useCredentialKeyReveal 状态机），本域的列表
 * 接口恒回明文（方案 §2 复制模型），复用会把两套数据源形态焊死（SRP）。
 *
 * revealed 状态由上层持有（page hook 的 revealedId）：桌面/移动两视图同时
 * 挂载，本地 state 会让两视图分叉——隐藏子树里滞留明文且「隐藏」按钮不可达。
 * 明文态 `translate="no"` 防整页翻译把明文上送翻译服务（沿号池纪律）；
 * 明文态不设 title——文本已全量可见，title 是第二份明文暴露面。
 */
import { Copy, Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { copySecretWithToast } from "@/lib/clipboard";
import type { ApiKeyItem } from "@/lib/api";

/**
 * 掩码短形。合法 key 最短 16（后端 MinCallerKeyValueLen），对短值做「前12…后4」
 * 截断几乎不遮（16 位值截完只剩 0 位实体隐藏），故 <24 位一律全遮——
 * 掩码态 DOM 永不出现完整明文。解密失败降级行（"****"）同样落入全遮分支。
 */
function shortKeyForm(plain: string): string {
  if (plain.length < 24) return "****";
  return `${plain.slice(0, 12)}…${plain.slice(-4)}`;
}

type ApiKeysKeyCellProps = {
  apiKey: ApiKeyItem;
  /** 显示态由上层单点持有，两视图共享 */
  revealed: boolean;
  onToggleReveal: (id: number) => void;
  /** xs = 移动卡片（更小字号与点击区），sm = 桌面表格 */
  size?: "sm" | "xs";
};

export function ApiKeysKeyCell({ apiKey, revealed, onToggleReveal, size = "sm" }: ApiKeysKeyCellProps) {
  const iconCls = size === "xs" ? "size-3" : "size-3.5";
  const textCls = size === "xs" ? "text-[11px]" : "text-xs";
  const buttonCls = size === "xs" ? "size-6" : "size-7";
  const display = revealed ? apiKey.api_key : shortKeyForm(apiKey.api_key);

  return (
    <div
      className={
        revealed
          ? "flex min-w-0 items-start gap-1"
          : size === "xs"
            ? "flex min-w-0 items-center gap-1"
            : "flex max-w-[240px] items-center gap-1"
      }
    >
      <span
        className={
          revealed
            ? `min-w-0 whitespace-normal break-all font-mono ${textCls}`
            : `min-w-0 truncate font-mono ${textCls}`
        }
        translate={revealed ? "no" : undefined}
        title={revealed ? undefined : display}
      >
        {display}
      </span>
      {/* 按钮不设 title：语义由 aria-label 承载（沿号池密集表格的决定） */}
      <div className="flex shrink-0 items-center">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={`${buttonCls} text-muted-foreground`}
          onClick={() => onToggleReveal(apiKey.id)}
          aria-label={revealed ? `隐藏密钥 ${apiKey.name}` : `显示密钥 ${apiKey.name} 的明文`}
        >
          {revealed ? <EyeOff className={iconCls} /> : <Eye className={iconCls} />}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={`${buttonCls} text-muted-foreground`}
          onClick={() => void copySecretWithToast(apiKey.api_key, `已复制密钥 ${apiKey.name}`)}
          aria-label={`复制密钥 ${apiKey.name}`}
        >
          <Copy className={iconCls} />
        </Button>
      </div>
    </div>
  );
}
