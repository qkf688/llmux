/** 关联层能力的三态展示值：true/false = 本链路 override，null = 继承模型真值 */
type CapabilityValue = boolean | null;

type ModelCaps = { tool_call: boolean; structured_output: boolean; image: boolean };

type CapabilityBadgesProps = {
  // 三态（null=继承模型真值）。必填不可选：后端保证该键始终存在，
  // 读成 undefined 会让继承态被误渲染成"无能力"。
  toolCall: CapabilityValue;
  structuredOutput: CapabilityValue;
  image: CapabilityValue;
  withHeader: boolean;
  // 模型侧固有能力真值：**参与渲染**，用于把继承项解析成最终能力——
  // 本列回答的是"这条链路能干什么"，故直接显示最终值；「继承还是显式」
  // 属于配置来源，在编辑弹窗里看，不该占列表列宽。
  //
  // 可选仅因上游可能拿不到（列表页未选中模型时 modelCaps 为 undefined）：
  // 此时继承项无法解析，降级为「继承模型」徽章——宁可说"继承中"，
  // 也不把解析不出来的继承项谎报成"无能力"。调用点（桌面表格/移动列表）必须传。
  modelCaps?: ModelCaps;
  emptyClassName?: string;
  // 默认允许换行（移动端卡片等宽度自由的容器）。列宽固定的表格单元格必须传 true：
  // 那里 wrap 会让行高随列宽抖动（同一张表内 2 倍行高差就是这么来的）。
  nowrap?: boolean;
};

// 徽章配色沿用各能力既有色位（与改动前一致）。
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

const CAP_MODEL_KEYS = {
  toolCall: "tool_call",
  structuredOutput: "structured_output",
  image: "image",
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
  const caps: { key: keyof typeof CAP_STYLES; value: CapabilityValue }[] = [
    { key: "toolCall", value: toolCall },
    { key: "structuredOutput", value: structuredOutput },
    { key: "image", value: image },
  ];

  // 解析最终能力 = override 优先，否则继承模型真值（与后端 COALESCE 同语义）。
  // 结果 true=支持 / false=不支持 / null=继承但模型未加载、无法解析。
  const resolved = caps.map(({ key, value }) => {
    if (value !== null) return { key, on: value };
    if (modelCaps) return { key, on: modelCaps[CAP_MODEL_KEYS[key]] };
    return { key, on: null as boolean | null };
  });
  const supported = resolved.filter((c) => c.on === true);
  const unresolvable = resolved.filter((c) => c.on === null);

  if (supported.length === 0 && unresolvable.length === 0 && !withHeader) {
    return <span className={emptyClassName}>无</span>;
  }

  return (
    <div className={`flex gap-1 ${nowrap ? "flex-nowrap" : "flex-wrap"}`}>
      {supported.map(({ key }) => (
        <span key={key} className={`${badgeCls} ${CAP_STYLES[key]}`} title={CAP_LABELS[key]}>
          {CAP_LABELS[key]}
        </span>
      ))}
      {/* 只在解析不出模型真值时出现：宁可说"继承中"，也不谎报"无能力" */}
      {unresolvable.length > 0 && (
        <span
          className={`${badgeCls} bg-muted text-muted-foreground`}
          title="该能力跟随模型设置，当前未加载到模型信息"
        >
          继承模型
        </span>
      )}
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
