import { Badge } from "@/components/ui/badge";
import type { ChatLog, MismatchedRequestFields, UnclaimedRequestFields, UnclaimedStatus } from "@/lib/api";

type SectionProps = {
  log: ChatLog;
};

type Tone = "warning" | "muted" | "danger";

type DiagnosticsView = {
  tone: Tone;
  headline: string;
  /** 补充说明，解释「为什么查不了」或「该怎么让它可查」 */
  description?: string;
  /** 待展示的命中键名，仅 ok 且非空时有内容 */
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

/** 两类诊断共用的形状（后端也是同一个 struct），差别只在文案。 */
type Diagnostics = UnclaimedRequestFields | MismatchedRequestFields;

const toneClass: Record<Tone, { box: string; text: string }> = {
  warning: { box: "bg-warning-tint", text: "text-warning-foreground" },
  muted: { box: "bg-muted/20", text: "text-muted-foreground" },
  danger: { box: "bg-destructive-tint", text: "text-destructive-tint-foreground" },
};

/**
 * 未认领键的四态呈现，用 Record 而非 switch：`UnclaimedStatus` 是字面量联合，
 * 少填一态编译期就报错——后端加第五态时不会静默漏渲染。
 */
const unclaimedViewByStatus: Record<UnclaimedStatus, (log: ChatLog, diagnostics: Diagnostics) => DiagnosticsView> = {
  ok: (_log, diagnostics) =>
    diagnostics.fields.length > 0
      ? {
          tone: "warning",
          headline: `本网关未解析的顶层键 (${diagnostics.fields.length})`,
          fields: diagnostics.fields,
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
    // 注意：三个生产 style 现已全部支持未认领检测，本分支**当前渲染不到**。保留是因为它是
    // 后端契约的一部分——新协议 style 落地时必然先经过未注册阶段，届时这是唯一能把
    // 「查不了」与「已检查、无未认领键」区分开的文案。清理死代码时不要删。
    description: "该协议的检测能力尚未实现，与本次请求是否正常无关。",
    fields: [],
    showBoundary: false,
  }),
  parse_error: (_log, diagnostics) => ({
    tone: "danger",
    headline: "检测失败：原始请求体不是 JSON 对象",
    description: "键的概念不成立，无法判断哪些顶层键未被认领。",
    fields: [],
    detail: diagnostics.detail,
    showBoundary: false,
  }),
};

/**
 * 类型不匹配键的四态呈现。文案与未认领各写一份而非共用：两者的成因与修法相反
 * （未认领 = 网关没实现该字段，类型不匹配 = 客户端传错类型），措辞混用会让用户
 * 不知道该改谁。
 */
const mismatchedViewByStatus: Record<UnclaimedStatus, (log: ChatLog, diagnostics: Diagnostics) => DiagnosticsView> = {
  ok: (_log, diagnostics) =>
    diagnostics.fields.length > 0
      ? {
          tone: "warning",
          headline: `因值类型不符被丢弃的顶层键 (${diagnostics.fields.length})`,
          description: "这些键本网关认得，但值的类型与协议不符，已被当作「未传」忽略——请求照常成功，参数不生效。",
          fields: diagnostics.fields,
          showBoundary: true,
        }
      : {
          tone: "muted",
          headline: "已完成检测：入站请求体的顶层键值类型均正确",
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
    headline: `不适用：${log.style} 入站请求不存在此类静默丢弃`,
    // 与未认领检测的同名状态语义不同：这里不是「能力缺口」，而是该协议对类型错误
    // 直接报错、根本没有可暴露的静默。措辞必须区分开，否则会被读成待补的 TODO。
    description: "该协议遇到类型不符的字段会直接返回错误，不会静默忽略，因此无需此项检测。",
    fields: [],
    showBoundary: false,
  }),
  parse_error: (_log, diagnostics) => ({
    tone: "danger",
    headline: "检测失败：原始请求体不是 JSON 对象",
    description: "键的概念不成立，无法判断哪些顶层键的值类型不符。",
    fields: [],
    detail: diagnostics.detail,
    showBoundary: false,
  }),
};

const boundaryNote = {
  unclaimed:
    "结果指「本网关 DTO 未认领的键」，不等于转换时丢失：入站 style 与上游一致时请求原样透传。检测只看顶层，messages[] 等嵌套字段不在范围内。",
  mismatched:
    "结果指「值类型不符而被容器忽略的键」，不等于上游拒绝了该参数。检测只看顶层，且只覆盖有三态的字段，messages[] 等嵌套字段不在范围内。",
};

function DiagnosticsSection({
  log,
  diagnostics,
  title,
  view,
  boundary,
}: {
  log: ChatLog;
  diagnostics?: Diagnostics;
  title: string;
  view: Record<UnclaimedStatus, (log: ChatLog, diagnostics: Diagnostics) => DiagnosticsView>;
  boundary: string;
}) {
  if (!diagnostics) {
    return null;
  }

  const resolved = view[diagnostics.status](log, diagnostics);
  const tone = toneClass[resolved.tone];

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</p>
      <div className={`rounded-md border ${tone.box} p-2 space-y-2 sm:p-3`}>
        <p className={`text-[11px] ${tone.text} uppercase tracking-wide`}>{resolved.headline}</p>
        {resolved.description && <p className="text-[11px] text-muted-foreground">{resolved.description}</p>}
        {resolved.detail && (
          <p className="break-all font-mono text-[11px] text-muted-foreground">{resolved.detail}</p>
        )}
        {resolved.fields.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {resolved.fields.map((field) => (
              <Badge key={field} variant="outline" className={`font-mono font-normal ${tone.text}`}>
                {field}
              </Badge>
            ))}
          </div>
        )}
        {resolved.showBoundary && <p className="text-[11px] text-muted-foreground">{boundary}</p>}
      </div>
    </div>
  );
}

/**
 * 展示入站原始请求体里本网关未认领的顶层键。
 *
 * 四态（已检测 / 原始体未记录 / 该协议不支持检测 / body 解析失败）各有文案，
 * **不许压成两态**——把「查不了」显示成「已检查、无未认领键」是假阴性，
 * 比没有这个功能更糟。字段整体缺席（include_raw=false）时才不渲染。
 *
 * 只列键名不带值预览：转换前 body 已在同一弹窗全文展示，且少一层敏感数据暴露面。
 */
export function UnclaimedFieldsSection({ log }: SectionProps) {
  return (
    <DiagnosticsSection
      log={log}
      diagnostics={log.unclaimed_request_fields}
      title="未认领请求字段"
      view={unclaimedViewByStatus}
      boundary={boundaryNote.unclaimed}
    />
  );
}

/**
 * 展示入站请求体里**被认领、但值类型不符而被静默丢弃**的顶层键。
 *
 * 与未认领分成两块而不是合并成一个列表：两者成因与修法相反（前者改网关、后者改客户端），
 * 混在一起用户无法判断该改谁；且两项检测的支持面不重合（openai-res 支持前者、不适用后者）。
 */
export function MismatchedFieldsSection({ log }: SectionProps) {
  return (
    <DiagnosticsSection
      log={log}
      diagnostics={log.mismatched_request_fields}
      title="类型不符被丢弃的请求字段"
      view={mismatchedViewByStatus}
      boundary={boundaryNote.mismatched}
    />
  );
}
