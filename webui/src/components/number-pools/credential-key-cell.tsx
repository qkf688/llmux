/**
 * 凭据 Key 单元格：桌面表格与移动卡片共用（同语义改动避免双轨漂移，先例 LogScheduleCell）。
 *
 * 默认渲染服务端掩码 KeyMasked；行内两个动作：
 * - 显示/隐藏：经受控 raw 端点取明文，明文生命周期由 useCredentialKeyReveal 管（翻页即作废）；
 * - 复制：直接写剪贴板，**不改变显示态**（未显示时静默取一次明文）。
 *
 * 明文态刻意解除限宽并允许折行：真实 key 普遍 40~100+ 字符，沿用掩码态的 truncate
 * 会让「显示」只露出前 1/4，等于没做这个功能。
 *
 * 本组件不含状态与请求，只按 reveal 快照渲染形态。
 */
import { Copy, Eye, EyeOff, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { CredentialKeyReveal } from "./use-credential-key-reveal";

type CredentialKeyCellProps = {
  credId: number;
  keyMasked: string;
  reveal: CredentialKeyReveal;
  /** xs = 移动卡片（更小字号与点击区），sm = 桌面表格 */
  size?: "sm" | "xs";
};

export function CredentialKeyCell({
  credId,
  keyMasked,
  reveal,
  size = "sm",
}: CredentialKeyCellProps) {
  const plaintext = reveal.plaintextOf(credId);
  const revealed = plaintext != null;
  const action = reveal.pendingAction(credId);
  const pending = action !== null;
  const iconCls = size === "xs" ? "size-3" : "size-3.5";
  const textCls = size === "xs" ? "text-[11px]" : "text-xs";
  const buttonCls = size === "xs" ? "size-6" : "size-7";
  // 掩码态限宽（table auto layout 下 td 不设宽会被内容撑开）；明文态放开宽度并折行，
  // items-start 让多行明文与按钮顶对齐
  const rootCls = revealed
    ? "flex min-w-0 items-start gap-1"
    : size === "xs"
      ? "flex min-w-0 items-center gap-1"
      : "flex max-w-[240px] items-center gap-1";

  return (
    <div className={rootCls}>
      <span
        className={
          revealed
            ? `min-w-0 whitespace-normal break-all font-mono ${textCls}`
            : `min-w-0 truncate font-mono ${textCls}`
        }
        // translate="no"：浏览器整页翻译会把可见文本上传翻译服务，明文不得出境
        translate={revealed ? "no" : undefined}
        title={revealed ? plaintext : keyMasked}
      >
        {revealed ? plaintext : keyMasked}
      </span>
      {/* 刻意不设 title：原生 hover 提示在密集表格里频繁弹出干扰阅读，按钮语义由 aria-label 承载 */}
      <div className="flex shrink-0 items-center">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={`${buttonCls} text-muted-foreground`}
          disabled={pending}
          onClick={() => void reveal.toggle(credId)}
          aria-label={revealed ? `隐藏 key ${keyMasked}` : `显示 key ${keyMasked} 的明文`}
        >
          {action === "reveal" ? (
            <Loader2 className={`${iconCls} animate-spin`} />
          ) : revealed ? (
            <EyeOff className={iconCls} />
          ) : (
            <Eye className={iconCls} />
          )}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={`${buttonCls} text-muted-foreground`}
          disabled={pending}
          onClick={() => void reveal.copy(credId, keyMasked)}
          aria-label={`复制 key ${keyMasked}`}
        >
          {action === "copy" ? (
            <Loader2 className={`${iconCls} animate-spin`} />
          ) : (
            <Copy className={iconCls} />
          )}
        </Button>
      </div>
    </div>
  );
}
