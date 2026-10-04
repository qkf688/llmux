import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ModelProviderTestResult } from "@/lib/api";
import type { ReactTestResultState } from "@/stores/model-providers/types";
import { TooltipProvider } from "@/components/ui/tooltip";

import { capabilityTestKey, type CapabilityTestResults } from "../../hooks/use-model-providers-testing";
import type { CapabilityTestKey, TestMode, TestType } from "../../types";
import { TestDialog } from "./test-dialog";

// 剪贴板三级降级链由 lib/clipboard 承载（其自身有测试），这里只钉本弹窗
// 「把什么交给它、什么状态下不给点」。
const { copyTextWithToastMock } = vi.hoisted(() => ({ copyTextWithToastMock: vi.fn() }));

vi.mock("@/lib/clipboard", () => ({
  copyTextWithToast: copyTextWithToastMock,
}));

type CapabilityTestEntry = CapabilityTestResults[CapabilityTestKey];

const TEST_ID = 42;
const MODE: TestMode = "non_stream";
const COPY_LABEL = "复制测试结果";
const REACT_IDLE: ReactTestResultState = { loading: false, messages: "", success: null, error: null };

const capabilityResult = (over: Partial<ModelProviderTestResult> = {}): ModelProviderTestResult => ({
  passed: true,
  mode: MODE,
  ...over,
});

// 返回注解工厂（禁 `as T`：as 会静默吞掉字段拼错）。
const entry = (over: Partial<CapabilityTestEntry> = {}): CapabilityTestEntry => ({
  loading: false,
  result: null,
  ...over,
});

const resultsOf = (value: CapabilityTestEntry): CapabilityTestResults => ({
  [capabilityTestKey(TEST_ID, MODE)]: value,
});

function renderDialog(props: {
  testType: TestType;
  testResults?: CapabilityTestResults;
  reactTestResult?: ReactTestResultState;
}) {
  const { testType, testResults = {}, reactTestResult = REACT_IDLE } = props;
  return render(
    <TooltipProvider>
      <TestDialog
        open
        onOpenChange={vi.fn()}
        testType={testType}
        onTestTypeChange={vi.fn()}
        testMode={MODE}
        onTestModeChange={vi.fn()}
        selectedTestId={TEST_ID}
        testResults={testResults}
        structuredTestResults={testResults}
        thinkingTestResults={testResults}
        visionTestResults={testResults}
        reactTestResult={reactTestResult}
        onClose={vi.fn()}
        onExecute={vi.fn()}
      />
    </TooltipProvider>
  );
}

function copyButton(): HTMLButtonElement {
  return screen.getByRole("button", { name: COPY_LABEL });
}

/** 点复制按钮并取回交出去的那段文本（点击是异步入口，一律 waitFor 收）。 */
async function copyOut(): Promise<string> {
  fireEvent.click(copyButton());
  await waitFor(() => expect(copyTextWithToastMock).toHaveBeenCalledTimes(1));
  return copyTextWithToastMock.mock.calls[0][0];
}

beforeEach(() => {
  copyTextWithToastMock.mockReset();
  copyTextWithToastMock.mockResolvedValue(true);
});

describe("模型测试：复制整份结果", () => {
  it("交出结论 + 各段正文 + 流式诊断四行，段名与屏幕一致", async () => {
    renderDialog({
      testType: "structured_output",
      testResults: resultsOf(
        entry({
          result: capabilityResult({
            parsed: { city: "南京" },
            raw_output: "{\"city\":\"南京\"}",
            stream: { frames: 3, first_frame_ms: 120, duration_ms: 1500, terminated_by: "[DONE]" },
          }),
        })
      ),
    });

    const text = await copyOut();

    expect(text).toContain("测试结论：通过");
    expect(text).toContain("【解析结果】");
    // 解析结果按两格缩进的 JSON 落盘，与结果区展示同一份字符串
    expect(text).toContain('"city": "南京"');
    expect(text).toContain("【原始输出】");
    expect(text).toContain("SSE 帧数：3");
    expect(text).toContain("首帧耗时：120 ms");
    expect(text).toContain("总耗时：1500 ms");
    expect(text).toContain("终止事件：[DONE]");
  });

  it("失败时带失败原因原文，段名用失败兜底语", async () => {
    renderDialog({
      testType: "connectivity",
      testResults: resultsOf(entry({ result: capabilityResult({ passed: false, error: "HTTP 状态码: 502" }) })),
    });

    const text = await copyOut();

    expect(text).toContain("测试结论：失败");
    expect(text).toContain("失败原因：HTTP 状态码: 502");
  });

  it("流式模式的原始输出段名是「原始 SSE 输出」", async () => {
    renderDialog({
      testType: "vision",
      testResults: resultsOf(
        entry({
          result: capabilityResult({ mode: "stream", vision_text: "红色", raw_output: "data: ..." }),
        })
      ),
    });

    const text = await copyOut();

    expect(text).toContain("【识别结果】");
    expect(text).toContain("【原始 SSE 输出】");
    expect(text).not.toContain("【原始输出】");
  });

  it("thinking 测试带推理内容段", async () => {
    renderDialog({
      testType: "thinking",
      testResults: resultsOf(entry({ result: capabilityResult({ thinking_text: "先拆步骤…" }) })),
    });

    expect(await copyOut()).toContain("【推理内容】");
  });

  it("ReAct 走「测试日志」段，取消后保留的半截日志一并带出", async () => {
    renderDialog({
      testType: "react",
      reactTestResult: {
        loading: false,
        success: false,
        error: null,
        messages: "[开始测试] deepseek-flash-free\n[调用工具] {\"name\":\"get_weather\"}\n",
      },
    });

    const text = await copyOut();

    expect(text).toContain("测试结论：失败");
    expect(text).toContain("【测试日志】");
    expect(text).toContain("[调用工具]");
  });

  it.each<[string, CapabilityTestEntry]>([
    ["未执行", entry()],
    ["执行中", entry({ loading: true })],
    ["已取消", entry({ cancelled: true })],
  ])("%s时按钮置灰，点不动也不复制", (_label, value) => {
    renderDialog({ testType: "connectivity", testResults: resultsOf(value) });

    const button = copyButton();
    expect(button.disabled).toBe(true);
    fireEvent.click(button);
    expect(copyTextWithToastMock).not.toHaveBeenCalled();
  });

  it("ReAct 未出结论时置灰（未执行 / 执行中 / 已取消且无结论）", () => {
    renderDialog({ testType: "react", reactTestResult: { ...REACT_IDLE, loading: true, messages: "…" } });

    const button = copyButton();
    expect(button.disabled).toBe(true);
    fireEvent.click(button);
    expect(copyTextWithToastMock).not.toHaveBeenCalled();
  });

  it("连通性测试没有正文段也仍可复制（结论本身就是全部）", async () => {
    renderDialog({ testType: "connectivity", testResults: resultsOf(entry({ result: capabilityResult() })) });

    expect(await copyOut()).toBe("测试结论：通过");
  });
});
