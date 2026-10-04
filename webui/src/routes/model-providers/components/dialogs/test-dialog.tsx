import { Copy } from "lucide-react";
import type { CapabilityTestKey, TestMode, TestType } from "../../types";
import type { ModelProviderTestResult } from "@/lib/api";
import type { ReactTestResultState } from "@/stores/model-providers/types";
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
  buildReactReport,
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
  reactTestResult: ReactTestResultState;
  onClose: () => void;
  onExecute: () => void;
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
];

// 执行模式：与测试类型正交（同一能力的两种跑法）。流式的判定输入是 SSE 折叠出的响应体，
// 通过标准与非流式完全一致，额外要求「真是 SSE 流 + 有内容增量 + 有终止事件」。
const TEST_MODE_OPTIONS: { value: TestMode; label: string; description: string }[] = [
  { value: "non_stream", label: "非流式", description: "一次性返回完整响应（默认，最快）" },
  {
    value: "stream",
    label: "流式",
    description: "带 stream=true：响应必须是 SSE 流、有内容增量、且以终止事件收尾。耗时明显更长。",
  },
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
function StreamStatsSection({ result }: { result: ModelProviderTestResult }) {
  const items = streamStatsItems(result);
  if (items.length === 0) {
    return null;
  }
  return (
    <div className="grid grid-cols-2 gap-x-3 gap-y-1 rounded-md border bg-background/40 p-2 text-xs sm:grid-cols-4">
      {items.map((item) => (
        <div key={item.label} className="flex flex-col">
          <span className="text-muted-foreground">{item.label}</span>
          <span className="font-mono">{item.value}</span>
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
  reactTestResult,
  onClose,
  onExecute,
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

  // 测试类型 → 是否执行中。Record 穷尽性保证新增 TestType 时这里被类型系统强制补一行，
  // 不再用嵌套三元（旧链的 else 分支会把未知类型误当 structured 的 loading）。
  const loadingByTestType: Record<TestType, boolean> = {
    connectivity: !!entryByCapabilityType.connectivity?.loading,
    react: reactTestResult.loading,
    structured_output: !!entryByCapabilityType.structured_output?.loading,
    thinking: !!entryByCapabilityType.thinking?.loading,
    vision: !!entryByCapabilityType.vision?.loading,
  };
  const executeDisabled = loadingByTestType[testType];

  // ReAct 端点自己发流（第三方 agent 库），模式开关对它没有意义——禁用而不是隐藏，
  // 是为了让「为什么这里不能选」有地方写说明，而不是留个凭空消失的控件。
  const modeSelectionDisabled = testType === "react" || executeDisabled;

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
  };
  const copyableReport = reportByTestType[testType]();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>模型测试</DialogTitle>
          <DialogDescription>选择要执行的测试类型</DialogDescription>
        </DialogHeader>

        <DialogBody className="space-y-4">
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
            {testType === "react" && <p className="text-xs text-muted-foreground">React 测试本身即流式，模式对其无影响。</p>}
          </div>

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

          {/* 固定高度结果区：切换测试类型 / 展开错误 / 追加日志时，dialog 外框尺寸恒定，仅此处内部滚动 */}
          <div className="h-64 overflow-y-auto rounded-md border bg-muted/20 p-3">
            <div className="mb-2 flex justify-end">
              <CopyResultButton report={copyableReport} />
            </div>

            {testType !== "react" && (
              <CapabilityResultSection
                spec={CAPABILITY_SPECS[testType]}
                entry={entryByCapabilityType[testType]}
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
