/**
 * 内联原文的体积策略（纯逻辑，无 JSX）：阈值、头尾预览切分、色调表。
 *
 * 与渲染组件分开是两层职责：**这里决定「给多少、怎么切」，组件只管「怎么显示」**。
 * 另外 react-refresh 规则要求组件文件只导出组件，常量/函数必须另置（见
 * body-text-block.tsx 的说明）。
 */

/**
 * 内联原文的体积闸：超过 MAX_PREVIEW_CHARS 的正文不整段进 DOM。
 *
 * 为什么要有这道闸（实测：同一 Chromium、768px 容器、1.98MB 单行 JSON 原文）：
 *   - `white-space: pre-wrap` 的 <pre> 渲染这段文本，**仅 layout 就吃掉 15s 主线程**
 *     （换行把 198 万字符切成 1.8 万个行盒逐行排版），日志详情弹窗直接无响应；
 *   - 同一段文本改成不换行（`white-space: pre` + 横向滚动）只要 **196ms**；
 *   - 截断到 64KB 再按换行渲染只要 **20ms**。
 * 结论：**贵的是「换行」，不是「字符多」**——字符内断行（break-words）不是主因
 * （去掉它同样要 15s）。所以：
 *   1. 默认视图只渲染头尾各 32KB（头尾都留：工具调用、工具结果这类关键片段常在
 *      正文两端，只留开头会把最该看的部分切掉）；
 *   2. 全文视图一律**不换行** + 横向滚动（保持原始文本结构不被换行打散）。
 */
export const MAX_PREVIEW_CHARS = 64 * 1024;

/** 块的色调：与原先三处容器的底色/字色一一对应（原始=info、转换后=success、头=muted）。 */
export type BodyTextTone = "muted" | "info" | "success";

export const TONE_CLASS: Record<BodyTextTone, { box: string; text: string }> = {
  muted: { box: "bg-muted/20", text: "text-muted-foreground" },
  info: { box: "bg-info-tint", text: "text-info-foreground" },
  success: { box: "bg-success-tint", text: "text-success-foreground" },
};

/**
 * 头尾预览：各取一半，中间用标记替代。
 *
 * 标记措辞与后端 `common.TruncateHeadTail` 同款（头尾保留、中间省略）——两侧
 * 是同一件事的两种实现（落库时截一次 / 展示时截一次），口径一致才好对账。
 */
export function buildHeadTailPreview(text: string): {
  content: string;
  head: number;
  tail: number;
  dropped: number;
} {
  const half = Math.floor(MAX_PREVIEW_CHARS / 2);
  const head = text.slice(0, half);
  const tail = text.slice(text.length - half);
  const dropped = text.length - head.length - tail.length;
  const marker = `……[内容过长已截断：显示开头 ${head.length.toLocaleString()} 字节、结尾 ${tail.length.toLocaleString()} 字节，中间省略 ${dropped.toLocaleString()} 字节]……`;
  return { content: `${head}${marker}${tail}`, head: head.length, tail: tail.length, dropped };
}
