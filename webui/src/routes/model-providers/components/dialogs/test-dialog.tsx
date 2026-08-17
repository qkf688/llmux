import type { TestType } from "../../types";
import type { ModelProviderTestResult } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { ExpandableError } from "@/components/expandable-error";
import { LoadingState } from "@/components/ui/loading-state";

type TestDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  testType: TestType;
  onTestTypeChange: (value: TestType) => void;
  selectedTestId: number | null;
  testResults: Record<number, { loading: boolean; result: ModelProviderTestResult | null }>;
  structuredTestResults: Record<number, { loading: boolean; result: ModelProviderTestResult | null }>;
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
];

// 日志/输出统一展示块：react 日志、结构化原始输出与解析结果三处复用。
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

export function TestDialog({
  open,
  onOpenChange,
  testType,
  onTestTypeChange,
  selectedTestId,
  testResults,
  structuredTestResults,
  reactTestResult,
  onClose,
  onExecute,
}: TestDialogProps) {
  const connectivityLoading = selectedTestId ? testResults[selectedTestId]?.loading : false;
  const structuredLoading = selectedTestId ? structuredTestResults[selectedTestId]?.loading : false;
  const executeDisabled =
    testType === "connectivity" ? connectivityLoading : testType === "react" ? reactTestResult.loading : structuredLoading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>模型测试</DialogTitle>
          <DialogDescription>选择要执行的测试类型</DialogDescription>
        </DialogHeader>

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

          {testType === "structured_output" &&
            (selectedTestId && structuredTestResults[selectedTestId]?.loading ? (
              <LoadingState text="测试中..." />
            ) : selectedTestId && structuredTestResults[selectedTestId]?.result ? (
              (() => {
                const result = structuredTestResults[selectedTestId]?.result;
                const passed = result?.passed === true;
                const errorType = result?.error_type;
                const errorMessage =
                  result?.error || result?.message || (passed ? "结构化输出能力测试通过" : "结构化输出能力测试失败");
                const rawOutput = result?.raw_output ?? "";
                const parsed = result?.parsed;

                return (
                  <div className="space-y-3">
                    <ExpandableError
                      error={{ message: errorMessage, summary: passed ? "测试成功" : "测试失败", type: errorType }}
                      isSuccess={passed}
                      defaultExpanded={!passed}
                    />
                    {rawOutput && <LogBlock label="原始输出" value={rawOutput} />}
                    {parsed != null && <LogBlock label="解析结果" value={JSON.stringify(parsed, null, 2)} wrap />}
                  </div>
                );
              })()
            ) : (
              <ResultPlaceholder />
            ))}
        </div>

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
