import { Badge } from "@/components/ui/badge";
import type { ChatLog, UnclaimedRequestFields, UnclaimedStatus } from "@/lib/api";

type UnclaimedFieldsSectionProps = {
  log: ChatLog;
};

type Tone = "warning" | "muted" | "danger";

type UnclaimedView = {
  tone: Tone;
  headline: string;
  /** 补充说明，解释「为什么查不了」或「该怎么让它可查」 */
  description?: string;
  /** 待展示的未认领键名，仅 ok 且非空时有内容 */
  fields: string[];
  /** 后端给出的解析错误原文，仅 parse_error 有 */
  detail?: string;
  /**
   * 是否展示语义边界脚注。只在**检测真的跑完**时展示：那时用户手里有一个结论，
   * 才存在把结论读成「转换已无问题」的风险；无从检测的三态没有结论可误读，
   * 挂上脚注只是噪音。
   */
  showBoundary: boolean;
};

const toneClass: Record<Tone, { box: string; text: string }> = {
  warning: { box: "bg-warning-tint", text: "text-warning-foreground" },
  muted: { box: "bg-muted/20", text: "text-muted-foreground" },
  danger: { box: "bg-destructive-tint", text: "text-destructive-tint-foreground" },
};

/**
 * 四态各自的呈现，用 Record 而非 switch：`UnclaimedStatus` 是字面量联合，
 * 少填一态编译期就报错——后端加第五态时不会静默漏渲染。
 */
const viewByStatus: Record<UnclaimedStatus, (log: ChatLog, unclaimed: UnclaimedRequestFields) => UnclaimedView> = {
  ok: (_log, unclaimed) =>
    unclaimed.fields.length > 0
      ? {
          tone: "warning",
          headline: `本网关未解析的顶层键 (${unclaimed.fields.length})`,
          fields: unclaimed.fields,
          showBoundary: true,
        }
      : {
          tone: "muted",
          // 必须说「已完成检测」而非「无问题」：与 raw_not_recorded 的「查不了」
          // 撞成同一句话，就是假阴性
          headline: "已完成检测：入站请求体的顶层键均被本网关解析",
          fields: [],
          showBoundary: true,
        },
  raw_not_recorded: () => ({
    tone: "muted",
    headline: "无从检测：本条日志未记录原始请求体",
    description: "检测的输入就是原始请求体。开启系统设置的「记录原始请求/响应体」后，新产生的日志才会带检测结果。",
    fields: [],
    showBoundary: false,
  }),
  style_unsupported: (log) => ({
    tone: "muted",
    headline: `无从检测：${log.style} 入站请求暂不支持此项检测`,
    // 措辞必须挡住「以为是 bug」的误读。不写「没有可反射的请求 DTO」这类实现术语：
    // 使用者读不懂，也无从据此行动；缺口的技术原因在 docs/architecture/modules/protocol-transform.md
    //
    // 注意：三个生产 style 现已全部支持检测，本分支**当前渲染不到**。保留是因为它是
    // 后端契约的一部分——新协议 style 落地时必然先经过未注册阶段，届时这是唯一能把
    // 「查不了」与「已检查、无未认领键」区分开的文案。清理死代码时不要删。
    description: "该协议的检测能力尚未实现，与本次请求是否正常无关。",
    fields: [],
    showBoundary: false,
  }),
  parse_error: (_log, unclaimed) => ({
    tone: "danger",
    headline: "检测失败：原始请求体不是 JSON 对象",
    description: "键的概念不成立，无法判断哪些顶层键未被认领。",
    fields: [],
    detail: unclaimed.detail,
    showBoundary: false,
  }),
};

/**
 * 展示入站原始请求体里本网关未认领的顶层键。
 *
 * 四态（已检测 / 原始体未记录 / 该协议不支持检测 / body 解析失败）各有文案，
 * **不许压成两态**——把「查不了」显示成「已检查、无未认领键」是假阴性，
 * 比没有这个功能更糟。字段整体缺席（include_raw=false）时才不渲染。
 *
 * 只列键名不带值预览：转换前 body 已在同一弹窗全文展示，且少一层敏感数据暴露面。
 */
export function UnclaimedFieldsSection({ log }: UnclaimedFieldsSectionProps) {
  const unclaimed = log.unclaimed_request_fields;
  if (!unclaimed) {
    return null;
  }

  const view = viewByStatus[unclaimed.status](log, unclaimed);
  const tone = toneClass[view.tone];

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">未认领请求字段</p>
      <div className={`rounded-md border ${tone.box} p-2 space-y-2 sm:p-3`}>
        <p className={`text-[11px] ${tone.text} uppercase tracking-wide`}>{view.headline}</p>
        {view.description && <p className="text-[11px] text-muted-foreground">{view.description}</p>}
        {view.detail && (
          <p className="break-all font-mono text-[11px] text-muted-foreground">{view.detail}</p>
        )}
        {view.fields.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {view.fields.map((field) => (
              <Badge key={field} variant="outline" className={`font-mono font-normal ${tone.text}`}>
                {field}
              </Badge>
            ))}
          </div>
        )}
        {view.showBoundary && (
          <p className="text-[11px] text-muted-foreground">
            结果指「本网关 DTO 未认领的键」，不等于转换时丢失：入站 style 与上游一致时请求原样透传。检测只看顶层，messages[]
            等嵌套字段不在范围内。
          </p>
        )}
      </div>
    </div>
  );
}
