import { cn } from "@/lib/utils";
import { Activity, Minus } from "lucide-react";

type StreamBadgeProps = {
  stream: boolean;
  /**
   * 列表行内嵌样式（inline）vs 详情弹窗 banner 样式（有底色、内边距）。
   * 颜色档：
   *   - inline  流式 = text-blue-600 / 非流式 = text-gray-500
   *   - banner  流式 = bg-blue-50 text-blue-700 / 非流式 = bg-gray-50 text-gray-700
   * 落点锁定下游契约（list + detail + 4 处 mock），改色档需同步本组件 + 测试断言。
   */
  variant?: "inline" | "banner";
  /** 行内模式下与同行其他文字对齐用的尺寸档（mobile 偏小）；banner 模式忽略 */
  textClassName?: string;
};

/**
 * 流式 / 非流式共享渲染：Activity（流式）/ Minus（非流式）+ 文字 + 颜色档。
 * 三处复用（desktop 列表 / mobile 列表 / detail 弹窗），统一字眼与颜色档。
 * 加 data-stream 属性供测试断言锁死两态（vitest 锁渲染输出与 className）。
 *
 * 字体来源：models.ChatLog.stream（建行时从 Before.Stream 拷贝），DTO 键恒在（false 也返）。
 * 存量行处理（Q6 拍板）：false 一律显"非流式"，不显示"未记录"提示。
 */
export function StreamBadge({ stream, variant = "inline", textClassName = "text-xs" }: StreamBadgeProps) {
  if (stream) {
    return (
      <span
        data-stream="true"
        data-variant={variant}
        className={cn(
          "inline-flex items-center gap-1",
          variant === "banner"
            ? "rounded-md px-2 py-1 bg-blue-50 text-blue-700 text-xs font-medium"
            : cn("text-blue-600 font-medium", textClassName),
        )}
      >
        <Activity className="size-4 shrink-0" aria-hidden="true" />
        <span>流式</span>
      </span>
    );
  }
  return (
    <span
      data-stream="false"
      data-variant={variant}
      className={cn(
        "inline-flex items-center gap-1",
        variant === "banner"
          ? "rounded-md px-2 py-1 bg-gray-50 text-gray-700 text-xs font-medium"
          : cn("text-gray-500", textClassName),
      )}
    >
      <Minus className="size-4 shrink-0" aria-hidden="true" />
      <span>非流式</span>
    </span>
  );
}