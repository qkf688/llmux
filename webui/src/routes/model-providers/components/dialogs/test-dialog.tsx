import { useState } from "react";
import { Copy } from "lucide-react";
import type { CapabilityTestKey, TestMode, TestType } from "../../types";
import type { ModelProviderTestResult, ProtocolProbeEntry } from "@/lib/api";
import { protocolLabel } from "@/lib/protocols";
import type { ReactTestResultState } from "@/stores/model-providers/types";
import type { ProtocolTestEntry, ProtocolTestResults } from "../../hooks/use-model-providers-testing";
import { copyTextWithToast } from "@/lib/clipboard";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { ExpandableError } from "@/components/expandable-error";
import { LoadingState } from "@/components/ui/loading-state";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { visionTestImageDataURL } from "../../vision-test-image";
// 复合键与结果表类型都从执行单元导入：键格式是「关联结果存储」的隐式契约，
// 两处各拼一遍 `${id}:${mode}` 会在改键时静默读空。
import { capabilityTestKey, type CapabilityTestResults } from "../../hooks/use-model-providers-testing";
// 段名 / 流式诊断四项数 / 复制文本拼装都在本模块定：结果区展示与剪贴板共用同一份。
import {
  buildCapabilityReport,
  buildProtocolReport,
  buildReactReport,
  formatDurationSeconds,
  protocolConfidenceLabel,
  protocolEvidenceChips,
  protocolName,
  protocolVerdictLabel,
  rawOutputLabel,
  streamStatsItems,
  type CapabilityTestSpec,
  type CapabilityTestType,
} from "./test-result-report";

// 结果按「关联:模式」存一格；键的构造与类型都来自执行单元（capabilityTestKey），
// 避免两处各拼一遍格式在改键时静默读空。

type CapabilityTestEntry = CapabilityTestResults[CapabilityTestKey];

type TestDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  testType: TestType;
  onTestTypeChange: (value: TestType) => void;
  testMode: TestMode;
  onTestModeChange: (value: TestMode) => void;
  selectedTestId: number | null;
  testResults: CapabilityTestResults;
  structuredTestResults: CapabilityTestResults;
  thinkingTestResults: CapabilityTestResults;
  visionTestResults: CapabilityTestResults;
  protocolTestResults: ProtocolTestResults;
  reactTestResult: ReactTestResultState;
  onClose: () => void;
  onExecute: () => void;
  // 协议检测的一键落地：把「允许集合 + 主格式」整项覆盖写入该关联的协议设置。
  // 由页面层持有写操作与列表刷新，弹窗只负责触发（不在这里直连 API）。
  onApplyProtocol?: (id: number, protocols: string[], primary: string) => Promise<void> | void;
  // 当前关联的协议集合是否处于继承态（null=继承供应商）。写入会把继承改成显式覆盖，
  // 之后供应商层再改协议，这条关联不再跟着变——这个状态变化必须在按钮旁说清。
  protocolsInherited?: boolean;
};

const TEST_OPTIONS: { value: TestType; label: string; description: string }[] = [
  { value: "connectivity", label: "连通性测试", description: "测试模型提供商的基本连通性" },
  { value: "react", label: "React Agent 能力测试", description: "测试模型的工具调用和反应能力" },
  {
    value: "structured_output",
    label: "结构化输出能力测试",
    description: "测试模型是否支持结构化输出（JSON Schema / Tool Output）",
  },
  {
    value: "thinking",
    label: "Thinking 能力测试",
    description:
      "测试模型是否支持推理（thinking）：请求携带 reasoning_effort / thinking block，响应须返回推理内容。可作为开启关联 thinking 开关的依据。",
  },
  {
    value: "vision",
    label: "图片识别能力测试",
    description:
      "测试模型是否支持视觉输入：内嵌 64×64 纯红色测试图并提问颜色，回答须识别出红色。可作为开启关联视觉能力的依据。",
  },
  {
    value: "protocol",
    label: "协议支持检测",
    description:
      "同一模型分别按 OpenAI / Responses / Anthropic 三种协议打上游：各协议的可用性、像不像原样直通的证据、原生字段完整度，并给出带置信度的建议。只读，不改任何配置。",
  },
];

// 执行模式：与测试类型正交（同一能力的两种跑法）。流式的判定输入是 SSE 折叠出的响应体，
// 通过标准与非流式完全一致，额外要求「真是 SSE 流 + 有内容增量 + 有终止事件」。
const TEST_MODE_OPTIONS: { value: TestMode; label: string; description: string }[] = [
  {
    value: "stream",
    label: "流式",
    description: "带 stream=true：响应必须是 SSE 流、有内容增量、且以终止事件收尾（默认）。耗时明显更长。",
  },
  { value: "non_stream", label: "非流式", description: "一次性返回完整响应（最快）" },
];

// 四类能力测试的段名与取值：屏幕上的段落与复制出去的文本从这张表取，两边不会各写一份而漂移。
// 新增能力测试类型 = 这里加一行（Record 穷尽性会强制补齐），不再逐类型抄 JSX（OCP + DRY）。
const CAPABILITY_SPECS: Record<CapabilityTestType, CapabilityTestSpec> = {
  connectivity: { passedLabel: "连通性测试通过", failedLabel: "连通性测试失败" },
  structured_output: {
    passedLabel: "结构化输出能力测试通过",
    failedLabel: "结构化输出能力测试失败",
    detail: {
      label: "解析结果",
      value: (result) => (result.parsed != null ? JSON.stringify(result.parsed, null, 2) : null),
    },
  },
  thinking: {
    passedLabel: "Thinking 能力测试通过",
    failedLabel: "Thinking 能力测试失败",
    detail: { label: "推理内容", value: (result) => result.thinking_text || null },
  },
  vision: {
    passedLabel: "图片识别能力测试通过",
    failedLabel: "图片识别能力测试失败",
    detail: { label: "识别结果", value: (result) => result.vision_text || null },
  },
};

// 日志/输出统一展示块：react 日志、各能力测试的原始输出/提取内容等处复用。
// wrap=false 时保留原始换行并横向滚动（适合日志/原始文本）；wrap=true 时自动换行（适合 JSON）。
function LogBlock({ label, value, wrap }: { label: string; value: string; wrap?: boolean }) {
  return (
    <div>
      <p className="text-xs font-medium text-muted-foreground mb-1">{label}</p>
      <pre
        className={cn(
          "rounded bg-background/60 p-2 font-mono text-xs",
          wrap ? "whitespace-pre-wrap break-words" : "overflow-x-auto whitespace-pre",
        )}
      >
        {value}
      </pre>
    </div>
  );
}

// 结果区未执行时的居中占位。
function ResultPlaceholder() {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      点击“执行测试”开始测试
    </div>
  );
}

// 流式诊断块：帧数 / 首帧耗时 / 总耗时 / 终止事件。四项的取值与复制文本共用
// streamStatsItems，改口径只改一处。
//
// 格子必须带 min-w-0：grid item 的 min-width 默认是 auto（= min-content），而「终止事件」的
// 值（response.completed 这类长 token）比同格的数字宽得多，格子按 min-content 撑开就会把值
// 顶出卡片右边界。值本身还要带 break-words：只让格子收缩而值不可断，长 token 照样溢出。
function StreamStatsSection({ result }: { result: ModelProviderTestResult }) {
  const items = streamStatsItems(result);
  if (items.length === 0) {
    return null;
  }
  return (
    <div className="grid grid-cols-2 gap-x-3 gap-y-1 rounded-md border bg-background/40 p-2 text-xs sm:grid-cols-4">
      {items.map((item) => (
        <div key={item.label} className="flex min-w-0 flex-col">
          <span className="text-muted-foreground">{item.label}</span>
          <span className="font-mono break-words">{item.value}</span>
        </div>
      ))}
    </div>
  );
}

// JSON 型能力测试（连通性 / 结构化输出 / thinking / vision）共用同一形状的回包：
// loading → 占位 → 结论 + 流式诊断 + 类型专属提取文本 + 原始输出。三态切换与段落取舍收敛进
// 本组件，新增测试类型不再逐块复制 JSX（DRY）。detail 取空时该块不渲染。
function CapabilityResultSection({
  spec,
  entry,
}: {
  spec: CapabilityTestSpec;
  entry: CapabilityTestEntry | undefined;
}) {
  if (entry?.loading) {
    return <LoadingState text="测试中..." />;
  }
  if (entry?.cancelled) {
    // 已取消独立成态：不显示成「失败」（不证明供应商不行），也不显示成「未测」。
    return (
      <div className="flex h-full flex-col items-center justify-center gap-1 text-sm text-muted-foreground">
        <span>已取消</span>
        <span className="text-xs">本次测试被中止，未产出结论</span>
      </div>
    );
  }
  const result = entry?.result ?? null;
  if (!result) {
    return <ResultPlaceholder />;
  }
  const passed = result.passed === true;
  const errorMessage = result.error || result.message || (passed ? spec.passedLabel : spec.failedLabel);
  const detail = spec.detail;
  const detailValue = detail?.value(result) ?? null;
  return (
    <div className="space-y-3">
      <ExpandableError
        error={{ message: errorMessage, summary: passed ? "测试成功" : "测试失败", type: result.error_type }}
        isSuccess={passed}
        defaultExpanded={!passed}
      />
      <StreamStatsSection result={result} />
      {detail && detailValue && <LogBlock label={detail.label} value={detailValue} wrap />}
      {result.raw_output && <LogBlock label={rawOutputLabel(result)} value={result.raw_output} />}
    </div>
  );
}

// 协议矩阵结果区：三行表格（协议 / 判定 / 非流式 / 流式 / 证据）+ 下方总评。
//
// 为什么是表格而不是卡片：协议检测的读法是「横向比较三条路」，卡片把同一条信息拆成三块
// 上下堆叠，三行就得滚半屏才看得全；表格一行一条协议，首屏就能比。
// 细节（URL、字段命中清单、失败原因、原生响应头）收进可展开区，默认不占地方。
//
// 协议名 / 判定名 / 证据行仍取自 test-result-report 的单一来源——屏幕与剪贴板同一份拼装。
function ProtocolResultSection({
  entry,
  associationId,
  onApply,
  protocolsInherited,
}: {
  entry: ProtocolTestEntry | undefined;
  associationId: number | null;
  onApply?: (id: number, protocols: string[], primary: string) => Promise<void> | void;
  protocolsInherited?: boolean;
}) {
  // 写操作在本组件内只持有「进行中」这一瞬时态；成败由页面层 toast 反馈。
  const [applying, setApplying] = useState(false);
  if (entry?.loading) {
    return <LoadingState text="检测中...（两段并发，最坏约 1.5 分钟）" />;
  }
  if (entry?.cancelled) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-1 text-sm text-muted-foreground">
        <span>已取消</span>
        <span className="text-xs">本次检测被中止，未产出结论</span>
      </div>
    );
  }
  const result = entry?.result;
  if (!result) {
    return <ResultPlaceholder />;
  }
  const protocols = result.protocols ?? [];
  const recommendation = result.recommendation;
  // 后端保证 protocols 是数组，但检测整体失败时旧版本会回 null——这里兜一层，
  // 免得一个空数组把整页带崩。
  const recommended = recommendation?.protocols ?? [];
  return (
    <div className="space-y-3">
      {protocols.length === 0 && (
        <p className="text-xs text-muted-foreground">{recommendation?.summary ?? "没有探测到任何协议"}</p>
      )}
      {protocols.length > 0 && (
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b text-left text-muted-foreground">
              <th className="py-1 pr-2 font-medium">协议</th>
              <th className="py-1 pr-2 font-medium">判定</th>
              <th className="py-1 pr-2 font-medium">非流式</th>
              <th className="py-1 font-medium">流式</th>
            </tr>
          </thead>
          {protocols.map((protocol) => (
            // 每条协议两行：第一行横向比较（协议/判定/两模式），第二行整行放证据短标签。
            // 证据不再单独占一列——窄弹窗里给它 60px 会挤成一字一行，行高比内容还高。
            <tbody key={protocol.protocol} className={cn(protocol.recommended && "bg-primary/5")}>
              <tr className="align-top">
                <td className="py-1.5 pr-2 whitespace-nowrap font-medium">{protocolName(protocol)}</td>
                <td className="py-1.5 pr-2">
                  <span
                    className={cn(
                      "rounded px-1.5 py-0.5",
                      protocol.verdict === "usable"
                        ? "bg-emerald-500/10 text-emerald-600"
                        : protocol.verdict === "partial"
                          ? "bg-amber-500/10 text-amber-600"
                          : "bg-destructive/10 text-destructive"
                    )}
                  >
                    {protocolVerdictLabel(protocol.verdict)}
                  </span>
                </td>
                <td className="py-1.5 pr-2 whitespace-nowrap">{protocolLegCell(protocol.non_stream)}</td>
                <td className="py-1.5 whitespace-nowrap">{protocolLegCell(protocol.stream)}</td>
              </tr>
              <tr className="border-b">
                <td colSpan={4} className="pb-1.5">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
                    {protocolEvidenceChips(protocol).map((chip) => (
                      <span key={chip} className="rounded bg-muted px-1 py-0.5 text-[11px]">
                        {chip}
                      </span>
                    ))}
                    <ProtocolDetail protocol={protocol} />
                  </div>
                </td>
              </tr>
            </tbody>
          ))}
        </table>
      )}
      {recommendation && (
        <div className="space-y-1 rounded-md border bg-background/40 p-2 text-xs">
          <p className="font-medium">
            {recommendation.summary}（置信度 {protocolConfidenceLabel(recommendation.confidence)}）
          </p>
          {recommendation.reason && <p className="text-muted-foreground">{recommendation.reason}</p>}
          {recommendation.actions.map((action) => (
            <p key={action}>· {action}</p>
          ))}
          {/* 一键落地的条件：后端给了可勾选集合（「无法区分」时仍给全部可用协议，只是不点名
              唯一主协议；可用协议全是转译路径时集合为空，此时不给按钮）。
              写的是「允许集合 + 主格式」而不是「只走一条」：多勾几条能让客户端用哪种协议进来
              就走哪条，省掉没必要的格式转换；主格式只在客户端发来集合外协议时决定转成哪种格式。 */}
          {recommended.length > 0 && associationId !== null && onApply && (
            <div className="flex flex-wrap items-center gap-2 pt-1">
              <Button
                size="sm"
                disabled={applying}
                onClick={() => {
                  setApplying(true);
                  // 先包一层 Promise 再调回调：回调若**同步抛错**，finally 仍会复位 loading。
                  // 失败提示由页面层（写操作 owner）负责，这里不重复 toast。
                  void Promise.resolve()
                    .then(() => onApply(associationId, recommended, recommendation.primary))
                    .finally(() => setApplying(false));
                }}
              >
                {applying
                  ? "写入中..."
                  : `一键勾选 ${recommended.length} 条可用协议（主格式 ${protocolLabel(recommendation.primary)}）`}
              </Button>
              <span className="text-muted-foreground">会替换该模型当前的协议设置</span>
              {/* 继承态被覆盖成显式设置是**单向**的：写入后供应商层再改协议，这条关联不再
                  跟着变。当下两条协议可能完全一致，用户看不出差别，所以必须在按钮旁点明。 */}
              {protocolsInherited && (
                <span className="text-muted-foreground">
                  当前为继承供应商，写入后将固定为该模型自己的设置
                </span>
              )}
            </div>
          )}
          {/* 三句免责是固定不变的同一段话，默认收起：它们在每次检测里都一样长，
              摊开会把「这次该怎么做」压到看不见；需要时点开仍在。 */}
          {recommendation.notes.length > 0 && (
            <details>
              <summary className="w-fit cursor-pointer list-none text-muted-foreground [&::-webkit-details-marker]:hidden">
                ▸ 说明（{recommendation.notes.length} 条）
              </summary>
              <div className="mt-1 space-y-0.5 text-muted-foreground">
                {recommendation.notes.map((note) => (
                  <p key={note}>· {note}</p>
                ))}
              </div>
            </details>
          )}
        </div>
      )}
    </div>
  );
}

// 单条探针腿的单元格：过=耗时，不过=失败原因短语（合规与否用符号区分，不靠颜色单独承载信息）。
function protocolLegCell(leg: ProtocolProbeEntry["non_stream"]) {
  const mark = leg.status === "passed" ? "✓" : leg.status === "noncompliant" ? "⚠" : "✗";
  const timing =
    leg.status === "passed"
      ? formatDurationSeconds(leg.duration_ms)
      : leg.summary || leg.detail || leg.error_type || "未通过";
  return (
    <span className="font-mono">
      {mark} {timing}
    </span>
  );
}

// 展开明细：URL 与来源、字段命中清单、两条腿的失败细节、原生响应头。
// 用原生 <details> 而非自造折叠控件：省一份状态与无障碍实现，键盘也能用。
function ProtocolDetail({ protocol }: { protocol: ProtocolProbeEntry }) {
  const legDetail = (label: string, leg: ProtocolProbeEntry["non_stream"]) => {
    if (leg.status === "passed") return null;
    return (
      <p className="text-muted-foreground">
        {label}：{leg.summary || "未通过"}
        {leg.detail ? ` — ${leg.detail}` : ""}
        {leg.terminated_by ? `（终止事件 ${leg.terminated_by}）` : ""}
      </p>
    );
  };
  // 收起时与证据标签同排（只占一个标签宽），展开时才独占一行，避免长句把标签行撑开。
  return (
    <details className="mt-1 [&[open]]:w-full [&[open]>summary>span]:rotate-90">
      {/* 与证据短标签同款外观：它本质是「多一条可选信息」，不该看起来像另一种控件。
          ▸ 是唯一的可点提示（收起时看不出可展开，用户不会去点）。 */}
      <summary className="w-fit cursor-pointer list-none rounded bg-muted px-1 py-0.5 text-[11px] [&::-webkit-details-marker]:hidden">
        <span className="inline-block transition-transform">▸</span> 明细
      </summary>
      <div className="mt-1 space-y-0.5 font-mono text-[11px]">
        <p className="break-all">
          {protocol.request_url}（{protocol.url_source === "endpoint" ? "已配置端点" : "继承 Base URL"}）
        </p>
        {/* 指纹请求本身的结论也要露出来：只说「指纹未采到」而不给状态码，用户没法判断
            是上游拒了假模型（正常）还是网关自己 5xx（本轮证据作废）。 */}
        <p>
          指纹请求：HTTP {protocol.fingerprint.status_code}
          {protocol.fingerprint.error_type ? `（${protocol.fingerprint.error_type}）` : ""}
        </p>
        {!protocol.fingerprint.same_family && protocol.fingerprint.message && <p>{protocol.fingerprint.message}</p>}
        {protocol.fields.hit.length > 0 && <p>原生字段命中：{protocol.fields.hit.join(", ")}</p>}
        {protocol.fields.miss.length > 0 && <p>缺失字段：{protocol.fields.miss.join(", ")}</p>}
        {/* 强证据单列：它是「像不像直通」的唯一正面依据，混在命中清单里读不出来。 */}
        {protocol.fields.strong_hit.length > 0 && (
          <p>不可伪造的原生特征：{protocol.fields.strong_hit.join(", ")}</p>
        )}
        {protocol.fingerprint.native_headers.length > 0 && (
          <p>原生响应头：{protocol.fingerprint.native_headers.join(", ")}</p>
        )}
        {legDetail("非流式", protocol.non_stream)}
        {legDetail("流式", protocol.stream)}
      </div>
    </details>
  );
}

// 复制整份测试结果：与内容同处一个结果框，按钮和它复制的东西不隔框。
// report 为 null = 本状态无结论可带走（未执行 / 已取消 / 测试中），置灰点不动；
// 置灰时不再挂 Tooltip——disabled 按钮收不到鼠标事件，悬停说明本来就弹不出来。
function CopyResultButton({ report }: { report: string | null }) {
  const button = (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className="size-7 text-muted-foreground hover:bg-muted hover:text-foreground"
      disabled={!report}
      onClick={() => {
        if (report) void copyTextWithToast(report, "已复制测试结果");
      }}
      aria-label="复制测试结果"
    >
      <Copy className="size-4" />
    </Button>
  );
  if (!report) {
    return button;
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent>复制本次测试的全部结果</TooltipContent>
    </Tooltip>
  );
}

export function TestDialog({
  open,
  onOpenChange,
  testType,
  onTestTypeChange,
  testMode,
  onTestModeChange,
  selectedTestId,
  testResults,
  structuredTestResults,
  thinkingTestResults,
  visionTestResults,
  protocolTestResults,
  reactTestResult,
  onClose,
  onExecute,
  onApplyProtocol,
  protocolsInherited,
}: TestDialogProps) {
  // 结果格按「关联:模式」定位：切换模式就是换一个格子，不会读到上一模式的结论。
  const entryOf = (map: CapabilityTestResults) =>
    selectedTestId ? map[capabilityTestKey(selectedTestId, testMode)] : undefined;
  const entryByCapabilityType: Record<CapabilityTestType, CapabilityTestEntry | undefined> = {
    connectivity: entryOf(testResults),
    structured_output: entryOf(structuredTestResults),
    thinking: entryOf(thinkingTestResults),
    vision: entryOf(visionTestResults),
  };

  // 协议检测没有模式轴：结果按关联 id 存一格，直接取。
  const protocolEntry = selectedTestId ? protocolTestResults[selectedTestId] : undefined;

  // 测试类型 → 是否执行中。Record 穷尽性保证新增 TestType 时这里被类型系统强制补一行，
  // 不再用嵌套三元（旧链的 else 分支会把未知类型误当 structured 的 loading）。
  const loadingByTestType: Record<TestType, boolean> = {
    connectivity: !!entryByCapabilityType.connectivity?.loading,
    react: reactTestResult.loading,
    structured_output: !!entryByCapabilityType.structured_output?.loading,
    thinking: !!entryByCapabilityType.thinking?.loading,
    vision: !!entryByCapabilityType.vision?.loading,
    protocol: !!protocolEntry?.loading,
  };
  const executeDisabled = loadingByTestType[testType];

  // ReAct 端点自己发流（第三方 agent 库），模式开关对它没有意义——禁用而不是隐藏，
  // 是为了让「为什么这里不能选」有地方写说明，而不是留个凭空消失的控件。
  // 协议检测同样没有模式轴：一次调用内部已把非流式与流式都跑完。
  const modeSelectionDisabled = testType === "react" || testType === "protocol" || executeDisabled;

  // 当前测试类型的整份复制文本（结论 + 逐段正文 + 流式诊断）。能力测试按格有无 result 判：
  // 该格没有 result 就是「未执行 / 执行中 / 已取消」三态，与结果区渲染占位的判据同源。
  const capabilityReportOf = (type: CapabilityTestType): string | null => {
    const result = entryByCapabilityType[type]?.result;
    return result ? buildCapabilityReport(CAPABILITY_SPECS[type], result) : null;
  };
  const reportByTestType: Record<TestType, () => string | null> = {
    connectivity: () => capabilityReportOf("connectivity"),
    react: () => buildReactReport(reactTestResult),
    structured_output: () => capabilityReportOf("structured_output"),
    thinking: () => capabilityReportOf("thinking"),
    vision: () => capabilityReportOf("vision"),
    protocol: () => buildProtocolReport(protocolEntry?.result ?? null),
  };
  const copyableReport = reportByTestType[testType]();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>模型测试</DialogTitle>
          <DialogDescription>选择要执行的测试类型</DialogDescription>
        </DialogHeader>

        {/* flex 列 + gap：结果区随内容增长，超出部分统一由 DialogBody（外框唯一滚动容器）消化。
            此前结果区写死 h-64 并自带滚动，形成「外框固定 + 小窗再滚」的双层滚动，
            三条协议的矩阵必须滚半屏才看得全。 */}
        <DialogBody className="flex flex-col gap-4">
          <RadioGroup
            value={testType}
            onValueChange={(value: string) => onTestTypeChange(value as TestType)}
            className="gap-2"
            disabled={executeDisabled}
          >
            {TEST_OPTIONS.map((opt) => (
              <label
                key={opt.value}
                htmlFor={opt.value}
                className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-md border p-3 transition-colors",
                  testType === opt.value ? "border-primary bg-primary/5" : "hover:bg-muted/50",
                )}
              >
                <RadioGroupItem value={opt.value} id={opt.value} className="mt-0.5" />
                <div className="space-y-0.5">
                  <span className="text-sm font-medium leading-none">{opt.label}</span>
                  <p className="text-xs text-muted-foreground">{opt.description}</p>
                </div>
              </label>
            ))}
          </RadioGroup>

          {/* 执行模式：协议检测**整块不渲染**（它没有模式轴，一次调用内部已把两种模式都跑完）。
              此前是「禁用 + 一行解释」，白白占掉约 120px 高度，还让人以为漏了开关。 */}
          {testType !== "protocol" && (
            <div className="space-y-2">
              <p className="text-xs font-medium text-muted-foreground">执行模式</p>
              <RadioGroup
                value={testMode}
                onValueChange={(value: string) => onTestModeChange(value as TestMode)}
                className="grid grid-cols-2 gap-2"
                disabled={modeSelectionDisabled}
              >
                {TEST_MODE_OPTIONS.map((opt) => (
                  <label
                    key={opt.value}
                    htmlFor={`mode-${opt.value}`}
                    className={cn(
                      "flex cursor-pointer items-start gap-2 rounded-md border p-2 transition-colors",
                      testMode === opt.value ? "border-primary bg-primary/5" : "hover:bg-muted/50",
                      modeSelectionDisabled && "cursor-not-allowed opacity-60",
                    )}
                  >
                    <RadioGroupItem value={opt.value} id={`mode-${opt.value}`} className="mt-0.5" />
                    <div className="space-y-0.5">
                      <span className="text-sm font-medium leading-none">{opt.label}</span>
                      <p className="text-xs text-muted-foreground">{opt.description}</p>
                    </div>
                  </label>
                ))}
              </RadioGroup>
              {testType === "react" && (
                <p className="text-xs text-muted-foreground">React 测试本身即流式，模式对其无影响。</p>
              )}
            </div>
          )}

          {/* vision 测试图预览：常量前端自内嵌（不经 API 回显），选中测试类型时即可看到「会给模型看什么图」 */}
          {testType === "vision" && (
            <div className="flex items-center gap-3 rounded-md border p-3">
              <img
                src={visionTestImageDataURL}
                width={64}
                height={64}
                alt="图片识别能力测试用图：64×64 纯红色"
                className="shrink-0 rounded border"
              />
              <p className="text-xs text-muted-foreground">
                测试时将发给模型的图片（64×64 纯色块）。模型回答识别出该颜色即判定通过。
              </p>
            </div>
          )}

          {/* 结果区：不写死高度、不自带滚动——内容多长它就长多长，超出外框上限时由 DialogBody 统一滚。
              data-slot 只给回归用例锚点：双层滚动正是「容器带固定高度 + 自带 overflow」造成的，
              没有任何行为断言能测到它，只能结构性盯住。 */}
          <div data-slot="test-result-area" className="rounded-md border bg-muted/20 p-3">
            <div className="mb-2 flex justify-end">
              <CopyResultButton report={copyableReport} />
            </div>

            {testType !== "react" && testType !== "protocol" && (
              <CapabilityResultSection
                spec={CAPABILITY_SPECS[testType]}
                entry={entryByCapabilityType[testType]}
              />
            )}

            {testType === "protocol" && (
              <ProtocolResultSection
                key={selectedTestId ?? "none"}
                entry={protocolEntry}
                associationId={selectedTestId}
                onApply={onApplyProtocol}
                protocolsInherited={protocolsInherited}
              />
            )}

            {testType === "react" &&
              (reactTestResult.loading ? (
                <LoadingState text="测试中..." />
              ) : reactTestResult.cancelled ||
                reactTestResult.error ||
                reactTestResult.success !== null ||
                reactTestResult.messages ? (
                <div className="space-y-3">
                  {reactTestResult.cancelled ? (
                    <div className="rounded-md border bg-muted/40 p-2 text-xs text-muted-foreground">
                      已取消：本次测试被中止，已保留中止前的日志
                    </div>
                  ) : reactTestResult.error ? (
                    <ExpandableError error={{ message: reactTestResult.error }} isSuccess={false} defaultExpanded />
                  ) : reactTestResult.success !== null ? (
                    <ExpandableError
                      error={{
                        message: reactTestResult.success ? "React Agent 能力测试通过" : "React Agent 能力测试失败",
                        summary: reactTestResult.success ? "测试成功" : "测试失败",
                      }}
                      isSuccess={reactTestResult.success}
                      defaultExpanded={false}
                    />
                  ) : null}

                  {reactTestResult.messages && <LogBlock label="测试日志" value={reactTestResult.messages} />}
                </div>
              ) : (
                <ResultPlaceholder />
              ))}
          </div>
        </DialogBody>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          <Button onClick={onExecute} disabled={executeDisabled}>
            {executeDisabled ? "测试中..." : "执行测试"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
