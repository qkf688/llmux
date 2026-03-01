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
        <span className="px-1.5 py-0.5 text-[10px] bg-blue-100 text-blue-700 rounded whitespace-nowrap">
          工具
        </span>
      )}
      {structuredOutput && (
        <span className="px-1.5 py-0.5 text-[10px] bg-purple-100 text-purple-700 rounded whitespace-nowrap">
          结构化
        </span>
      )}
      {image && (
        <span className="px-1.5 py-0.5 text-[10px] bg-green-100 text-green-700 rounded whitespace-nowrap">
          视觉
        </span>
      )}
      {withHeader && (
        <span className="px-1.5 py-0.5 text-[10px] bg-orange-100 text-orange-700 rounded whitespace-nowrap">
          透传
        </span>
      )}
    </div>
  );
}

