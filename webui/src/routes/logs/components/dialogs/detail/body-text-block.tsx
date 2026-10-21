import { useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { formatByteLength } from "../../../utils/formatters";
import { buildHeadTailPreview, MAX_PREVIEW_CHARS, TONE_CLASS, type BodyTextTone } from "./body-text-policy";

type BodyTextBlockProps = {
  /** 块标题（如「原始请求体 - 转换前」） */
  label: string;
  /** 原文正文 */
  text: string;
  /** 色调，默认中性 */
  tone?: BodyTextTone;
};

/**
 * 原文展示块：标题 + 大小 + 正文 +（超限时）截断说明与「查看全文」开关。
 *
 * 唯一的职责是**把任意大小的原文安全地渲染出来**：体积策略（阈值/头尾切分）在
 * body-text-policy.ts，这里只管呈现。调用方只给标题与正文，不再各自写一份
 * <pre> 样式（六个正文字段原先是六份复制粘贴的 className，改一次样式要改六处）。
 *
 * 本文件只导出组件：react-refresh 规则要求组件文件不混出常量与函数，故
 * MAX_PREVIEW_CHARS 等从 body-text-policy.ts 引入而不是就地导出。
 */
export function BodyTextBlock({ label, text, tone = "muted" }: BodyTextBlockProps) {
  const [expanded, setExpanded] = useState(false);
  const toneClass = TONE_CLASS[tone];
  const oversized = text.length > MAX_PREVIEW_CHARS;
  const preview = oversized ? buildHeadTailPreview(text) : null;
  const shown = expanded ? text : (preview ? preview.content : text);

  return (
    <div className={`rounded-md border ${toneClass.box} p-2 space-y-1 sm:p-3`}>
      <p className={`text-[11px] ${toneClass.text} uppercase tracking-wide`}>
        {label} ({formatByteLength(text)})
      </p>
      <pre
        className={cn(
          "text-xs font-mono max-h-40 overflow-y-auto",
          toneClass.text,
          // 全文不换行（排版成本的两个数量级差异见 body-text-policy.ts 的注释）；
          // 预览仍换行，32KB 换行渲染只要 20ms，可读性优先。
          expanded ? "whitespace-pre overflow-x-auto" : "whitespace-pre-wrap break-words"
        )}
      >
        {shown}
      </pre>
      {preview && (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-[11px] text-muted-foreground">
            {expanded
              ? "全文按原始文本渲染、不自动换行，横向滚动查看；也可点「导出」在编辑器中打开。"
              : `内容过大，已截断显示开头 ${preview.head.toLocaleString()} 字节与结尾 ${preview.tail.toLocaleString()} 字节，中间 ${preview.dropped.toLocaleString()} 字节已省略。`}
          </p>
          <Button variant="outline" size="sm" onClick={() => setExpanded((prev) => !prev)}>
            {expanded ? "收起全文" : "查看全文"}
          </Button>
        </div>
      )}
    </div>
  );
}
