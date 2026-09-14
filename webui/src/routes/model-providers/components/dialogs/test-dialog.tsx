import type { ReactNode } from "react";
import type { TestType } from "../../types";
import type { ModelProviderTestResult } from "@/lib/api";
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
import { visionTestImageDataURL } from "../../vision-test-image";

type TestResultsById = Record<number, { loading: boolean; result: ModelProviderTestResult | null }>;

type TestDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  testType: TestType;
  onTestTypeChange: (value: TestType) => void;
  selectedTestId: number | null;
  testResults: TestResultsById;
  structuredTestResults: TestResultsById;
  thinkingTestResults: TestResultsById;
  visionTestResults: TestResultsById;
  reactTestResult: {
    loading: boolean;
    messages: string;
    success: boolean | null;
    error: string | null;
  };
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

// JSON 型能力测试（结构化输出 / thinking / vision）共用同一形状的回包：
// loading → 占位 → 结论 + 类型专属提取文本 + 原始输出。三态切换收敛进本组件，
// 新增测试类型不再逐块复制 JSX（DRY）。renderDetail 返回 null 时不渲染该块。
function CapabilityResultSection({
  loading,
  result,
  passedLabel,
  failedLabel,
  renderDetail,
}: {
  loading: boolean;
  result: ModelProviderTestResult | null;
  passedLabel: string;
  failedLabel: string;
  renderDetail?: (result: ModelProviderTestResult) => ReactNode;
}) {
  if (loading) {
    return <LoadingState text="测试中..." />;
  }
  if (!result) {
    return <ResultPlaceholder />;
  }
  const passed = result.passed === true;
  const errorMessage = result.error || result.message || (passed ? passedLabel : failedLabel);
  const rawOutput = result.raw_output ?? "";
  return (
    <div className="space-y-3">
      <ExpandableError
        error={{ message: errorMessage, summary: passed ? "测试成功" : "测试失败", type: result.error_type }}
        isSuccess={passed}
        defaultExpanded={!passed}
      />
      {renderDetail ? renderDetail(result) : null}
      {rawOutput && <LogBlock label="原始输出" value={rawOutput} />}
    </div>
  );
}

export function TestDialog({
  open,
  onOpenChange,
  testType,
  onTestTypeChange,
  selectedTestId,
  testResults,
  structuredTestResults,
  thinkingTestResults,
  visionTestResults,
  reactTestResult,
  onClose,
  onExecute,
}: TestDialogProps) {
  const entryOf = (map: TestResultsById) => (selectedTestId ? map[selectedTestId] : undefined);
  const structuredEntry = entryOf(structuredTestResults);
  const thinkingEntry = entryOf(thinkingTestResults);
  const visionEntry = entryOf(visionTestResults);

  // 测试类型 → 是否执行中。Record 穷尽性保证新增 TestType 时这里被类型系统强制补一行，
  // 不再用嵌套三元（旧链的 else 分支会把未知类型误当 structured 的 loading）。
  const loadingByTestType: Record<TestType, boolean> = {
    connectivity: !!(selectedTestId && testResults[selectedTestId]?.loading),
    react: reactTestResult.loading,
    structured_output: !!structuredEntry?.loading,
    thinking: !!thinkingEntry?.loading,
    vision: !!visionEntry?.loading,
  };
  const executeDisabled = loadingByTestType[testType];

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
            {testType === "connectivity" &&
              (selectedTestId && testResults[selectedTestId]?.loading ? (
                <LoadingState text="测试中..." />
              ) : selectedTestId && testResults[selectedTestId] ? (
                <ExpandableError
                  error={{
                    message:
                      testResults[selectedTestId].result?.error ||
                      testResults[selectedTestId].result?.message ||
                      "测试成功",
                    summary: testResults[selectedTestId].result?.error ? "测试失败" : "测试成功",
                    type: testResults[selectedTestId].result?.error_type,
                  }}
                  isSuccess={!testResults[selectedTestId].result?.error}
                  defaultExpanded={!!testResults[selectedTestId].result?.error}
                />
              ) : (
                <ResultPlaceholder />
              ))}

            {testType === "react" &&
              (reactTestResult.loading ? (
                <LoadingState text="测试中..." />
              ) : reactTestResult.error || reactTestResult.success !== null || reactTestResult.messages ? (
                <div className="space-y-3">
                  {reactTestResult.error ? (
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

            {testType === "structured_output" && (
              <CapabilityResultSection
                loading={!!structuredEntry?.loading}
                result={structuredEntry?.result ?? null}
                passedLabel="结构化输出能力测试通过"
                failedLabel="结构化输出能力测试失败"
                renderDetail={(result) =>
                  result.parsed != null ? (
                    <LogBlock label="解析结果" value={JSON.stringify(result.parsed, null, 2)} wrap />
                  ) : null
                }
              />
            )}

            {testType === "thinking" && (
              <CapabilityResultSection
                loading={!!thinkingEntry?.loading}
                result={thinkingEntry?.result ?? null}
                passedLabel="Thinking 能力测试通过"
                failedLabel="Thinking 能力测试失败"
                renderDetail={(result) =>
                  result.thinking_text ? <LogBlock label="推理内容" value={result.thinking_text} wrap /> : null
                }
              />
            )}

            {testType === "vision" && (
              <CapabilityResultSection
                loading={!!visionEntry?.loading}
                result={visionEntry?.result ?? null}
                passedLabel="图片识别能力测试通过"
                failedLabel="图片识别能力测试失败"
                renderDetail={(result) =>
                  result.vision_text ? <LogBlock label="识别结果" value={result.vision_text} wrap /> : null
                }
              />
            )}
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
