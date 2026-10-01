type CapabilityBadgesProps = {
  toolCall: boolean;
  structuredOutput: boolean;
  image: boolean;
  withHeader: boolean;
  emptyClassName?: string;
  // 默认允许换行（移动端卡片等宽度自由的容器）。列宽固定的表格单元格必须传 true：
  // 那里 wrap 会让行高随列宽抖动（同一张表内 2 倍行高差就是这么来的）。
  nowrap?: boolean;
};

export function CapabilityBadges({
  toolCall,
  structuredOutput,
  image,
  withHeader,
  emptyClassName = "text-xs text-muted-foreground",
  nowrap = false,
}: CapabilityBadgesProps) {
  const hasAny = toolCall || structuredOutput || image || withHeader;

  if (!hasAny) {
    return <span className={emptyClassName}>无</span>;
  }

  return (
    <div className={`flex gap-1 ${nowrap ? "flex-nowrap" : "flex-wrap"}`}>
      {toolCall && (
        <span className="px-1.5 py-0.5 text-[10px] bg-[color:var(--chart-5)]/15 text-[color:var(--chart-5)] rounded whitespace-nowrap">
          工具
        </span>
      )}
      {structuredOutput && (
        <span className="px-1.5 py-0.5 text-[10px] bg-[color:var(--chart-9)]/15 text-[color:var(--chart-9)] rounded whitespace-nowrap">
          结构化
        </span>
      )}
      {image && (
        <span className="px-1.5 py-0.5 text-[10px] bg-[color:var(--chart-1)]/15 text-[color:var(--chart-1)] rounded whitespace-nowrap">
          视觉
        </span>
      )}
      {withHeader && (
        <span className="px-1.5 py-0.5 text-[10px] bg-[color:var(--chart-7)]/15 text-[color:var(--chart-7)] rounded whitespace-nowrap">
          透传
        </span>
      )}
    </div>
  );
}

