type CapabilityBadgesProps = {
  toolCall: boolean;
  structuredOutput: boolean;
  image: boolean;
  withHeader: boolean;
  emptyClassName?: string;
};

export function CapabilityBadges({
  toolCall,
  structuredOutput,
  image,
  withHeader,
  emptyClassName = "text-xs text-muted-foreground",
}: CapabilityBadgesProps) {
  const hasAny = toolCall || structuredOutput || image || withHeader;

  if (!hasAny) {
    return <span className={emptyClassName}>无</span>;
  }

  return (
    <div className="flex flex-wrap gap-1">
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

