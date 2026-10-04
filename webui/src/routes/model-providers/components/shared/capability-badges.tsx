/** 关联层能力的三态展示值：true/false = 本链路 override，null = 继承模型真值 */
type CapabilityValue = boolean | null;

type CapabilityBadgesProps = {
  // 三态（null=继承模型真值）。必填不可选：后端保证该键始终存在，
  // 读成 undefined 会让继承态被误渲染成"无能力"。
  toolCall: CapabilityValue;
  structuredOutput: CapabilityValue;
  image: CapabilityValue;
  withHeader: boolean;
  // 模型侧的固有能力真值：仅用于把继承态显示成"继承（启用/禁用）"，
  // 不传时继承态只显示「继承」二字。
  modelCaps?: { tool_call: boolean; structured_output: boolean; image: boolean };
  emptyClassName?: string;
  // 默认允许换行（移动端卡片等宽度自由的容器）。列宽固定的表格单元格必须传 true：
  // 那里 wrap 会让行高随列宽抖动（同一张表内 2 倍行高差就是这么来的）。
  nowrap?: boolean;
};

// 徽章配色沿用各能力既有色位（与改动前一致，只多了 inherit 态的中性色）。
const CAP_STYLES = {
  toolCall: "bg-[color:var(--chart-5)]/15 text-[color:var(--chart-5)]",
  structuredOutput: "bg-[color:var(--chart-9)]/15 text-[color:var(--chart-9)]",
  image: "bg-[color:var(--chart-1)]/15 text-[color:var(--chart-1)]",
} as const;

const CAP_LABELS = {
  toolCall: "工具",
  structuredOutput: "结构化",
  image: "视觉",
} as const;

const badgeCls = "px-1.5 py-0.5 text-[10px] rounded whitespace-nowrap";

export function CapabilityBadges({
  toolCall,
  structuredOutput,
  image,
  withHeader,
  modelCaps,
  emptyClassName = "text-xs text-muted-foreground",
  nowrap = false,
}: CapabilityBadgesProps) {
  const caps: { key: keyof typeof CAP_STYLES; value: CapabilityValue; modelKey: keyof NonNullable<typeof modelCaps> }[] = [
    { key: "toolCall", value: toolCall, modelKey: "tool_call" },
    { key: "structuredOutput", value: structuredOutput, modelKey: "structured_output" },
    { key: "image", value: image, modelKey: "image" },
  ];
  const hasAny = caps.some((c) => c.value !== null && c.value !== false) || withHeader;
  const anyInherited = caps.some((c) => c.value === null);

  if (!hasAny && !anyInherited) {
    return <span className={emptyClassName}>无</span>;
  }

  return (
    <div className={`flex gap-1 ${nowrap ? "flex-nowrap" : "flex-wrap"}`}>
      {caps.map(({ key, value, modelKey }) => {
        if (value === null) {
          // 继承态：把模型侧真值一并显示出来，否则「继承」二字看不出最终能力。
          const inheritedLabel = modelCaps ? (modelCaps[modelKey] ? "启用" : "禁用") : null;
          return (
            <span
              key={key}
              className={`${badgeCls} bg-muted text-muted-foreground`}
              title={`继承模型能力${inheritedLabel ? `（当前：${inheritedLabel}）` : ""}`}
            >
              {CAP_LABELS[key]}·继承{inheritedLabel ? `（${inheritedLabel}）` : ""}
            </span>
          );
        }
        if (!value) return null;
        return (
          <span key={key} className={`${badgeCls} ${CAP_STYLES[key]}`} title="本链路显式启用">
            {CAP_LABELS[key]}
          </span>
        );
      })}
      {withHeader && (
        <span
          className={`${badgeCls} bg-[color:var(--chart-7)]/15 text-[color:var(--chart-7)]`}
          title="透传自定义请求头"
        >
          透传
        </span>
      )}
    </div>
  );
}
