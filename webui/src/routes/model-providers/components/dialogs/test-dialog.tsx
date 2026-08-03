import type { TestType } from "../../types";
import type { ModelProviderTestResult } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Textarea } from "@/components/ui/textarea";
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
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto overflow-x-hidden">
        <DialogHeader>
          <DialogTitle>模型测试</DialogTitle>
          <DialogDescription>选择要执行的测试类型</DialogDescription>
        </DialogHeader>

        <RadioGroup
          value={testType}
          onValueChange={(value: string) => onTestTypeChange(value as TestType)}
          className="space-y-4"
        >
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="connectivity" id="connectivity" />
            <Label htmlFor="connectivity">连通性测试</Label>
          </div>
          <p className="text-sm text-muted-foreground ml-6">测试模型提供商的基本连通性</p>

          <div className="flex items-center space-x-2">
            <RadioGroupItem value="react" id="react" />
            <Label htmlFor="react">React Agent 能力测试</Label>
          </div>
          <p className="text-sm text-muted-foreground ml-6">测试模型的工具调用和反应能力</p>

          <div className="flex items-center space-x-2">
            <RadioGroupItem value="structured_output" id="structured_output" />
            <Label htmlFor="structured_output">结构化输出能力测试</Label>
          </div>
          <p className="text-sm text-muted-foreground ml-6">测试模型是否支持结构化输出（JSON Schema / Tool Output）</p>
        </RadioGroup>

        {testType === "connectivity" && (
          <div className="mt-4">
            {selectedTestId && testResults[selectedTestId]?.loading ? (
              <LoadingState text="测试中..." />
            ) : selectedTestId && testResults[selectedTestId] ? (
              <ExpandableError
                error={{
                  message:
                    testResults[selectedTestId].result?.error ||
                    testResults[selectedTestId].result?.message ||
                    "测试成功",
                }}
                isSuccess={!testResults[selectedTestId].result?.error}
                defaultExpanded={!!testResults[selectedTestId].result?.error}
              />
            ) : (
              <p className="text-muted-foreground">点击"执行测试"开始测试</p>
            )}
          </div>
        )}

        {testType === "react" && (
          <div className="mt-4 max-h-96 min-w-0">
            {reactTestResult.loading ? (
              <LoadingState text="测试中..." />
            ) : reactTestResult.error ? (
              <ExpandableError
                error={{ message: reactTestResult.error }}
                isSuccess={false}
                defaultExpanded
              />
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

            {reactTestResult.messages && (
              <div className="mt-4">
                <p className="text-xs font-medium text-muted-foreground mb-1">测试日志</p>
                <Textarea
                  name="logs"
                  className="max-h-48 resize-none whitespace-pre overflow-x-auto font-mono text-xs"
                  readOnly
                  value={reactTestResult.messages}
                />
              </div>
            )}
          </div>
        )}

        {testType === "structured_output" && (
          <div className="mt-4 min-w-0 pb-4">
            {selectedTestId && structuredTestResults[selectedTestId]?.loading ? (
              <LoadingState text="测试中..." />
            ) : selectedTestId && structuredTestResults[selectedTestId]?.result ? (
              (() => {
                const result = structuredTestResults[selectedTestId]?.result as Record<string, unknown> | null;
                const passed = result?.passed === true;
                const errorMessage =
                  (typeof result?.error === "string" && result.error) ||
                  (typeof result?.message === "string" && result.message) ||
                  (passed ? "结构化输出能力测试通过" : "结构化输出能力测试失败");
                const rawOutput = typeof result?.raw_output === "string" ? result.raw_output : "";
                const parsed = result?.parsed;

                return (
                  <div className="space-y-6">
                    <ExpandableError
                      error={{
                        message: errorMessage,
                        summary: passed ? "测试成功" : "测试失败",
                      }}
                      isSuccess={passed}
                      defaultExpanded={!passed}
                    />

                    {rawOutput && (
                      <div>
                        <p className="text-xs font-medium text-muted-foreground mb-1">原始输出</p>
                        <Textarea
                          name="raw_output"
                          className="max-h-48 resize-none whitespace-pre overflow-x-auto font-mono text-xs bg-background/50"
                          readOnly
                          value={rawOutput}
                        />
                      </div>
                    )}

                    {parsed != null && (
                      <div>
                        <p className="text-xs font-medium text-muted-foreground mb-1">解析结果</p>
                        <Textarea
                          name="parsed"
                          className="max-h-48 resize-none whitespace-pre overflow-x-auto font-mono text-xs bg-background/50"
                          readOnly
                          value={JSON.stringify(parsed, null, 2)}
                        />
                      </div>
                    )}
                  </div>
                );
              })()
            ) : (
              <p className="text-muted-foreground">点击"执行测试"开始测试</p>
            )}
          </div>
        )}

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
