import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ModelProviderTestResult, ProtocolProbeEntry, ProtocolProbeResult } from "@/lib/api";
import type { ReactTestResultState } from "@/stores/model-providers/types";
import type { ProtocolTestResults } from "../../hooks/use-model-providers-testing";
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
  protocolTestResults?: ProtocolTestResults;
  reactTestResult?: ReactTestResultState;
  onApplyProtocol?: (id: number, protocols: string[], primary: string) => Promise<void> | void;
  protocolsInherited?: boolean;
}) {
  const { testType, testResults = {}, protocolTestResults = {}, reactTestResult = REACT_IDLE, onApplyProtocol, protocolsInherited } = props;
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
        protocolTestResults={protocolTestResults}
        reactTestResult={reactTestResult}
        onClose={vi.fn()}
        onExecute={vi.fn()}
        onApplyProtocol={onApplyProtocol}
        protocolsInherited={protocolsInherited}
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

  // 流式诊断块曾把「终止事件」的值顶出卡片右边界：4 等分网格的格子 min-width 默认 auto
  // （= min-content），而 response.completed 这类长 mono token 比同格数字宽得多，格子按
  // min-content 撑开就把值挤到边框外。两条约束缺一不可：格子要能收缩（min-w-0）、
  // 值要能换行（break-words）——只给其中一条仍会溢出。
  it("流式诊断的格子可收缩、值可换行（长终止事件不撑出卡片）", () => {
    renderDialog({
      testType: "connectivity",
      testResults: resultsOf(
        entry({
          result: capabilityResult({
            mode: "stream",
            stream: { frames: 44, first_frame_ms: 3282, duration_ms: 3283, terminated_by: "response.completed" },
          }),
        })
      ),
    });

    const value = screen.getByText("response.completed");
    // 值本身要允许在格内换行（否则长 token 顶出边框）。
    expect(value.className).toContain("break-words");
    // 格子要允许收缩到轨道宽度（grid item 的 min-width:auto 会让它按 min-content 撑开）。
    const cell = value.parentElement;
    expect(cell?.className).toContain("min-w-0");
  });

  it("连通性测试没有正文段也仍可复制（结论本身就是全部）", async () => {
    renderDialog({ testType: "connectivity", testResults: resultsOf(entry({ result: capabilityResult() })) });

    expect(await copyOut()).toBe("测试结论：通过");
  });

  it("协议检测：矩阵逐行展示判定与证据，总评写明置信度", () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
    });

    // 三条协议都在（协议名来自 lib/protocols 的单一来源）。
    expect(screen.getByText("OpenAI")).toBeInTheDocument();
    expect(screen.getByText("OpenAI Responses")).toBeInTheDocument();
    expect(screen.getByText("Anthropic ★")).toBeInTheDocument();
    // 部分支持与不可用是两种结论，必须都在屏幕上可见。
    expect(screen.getByText("部分支持")).toBeInTheDocument();
    expect(screen.getByText("不可用")).toBeInTheDocument();
    // 总评带置信度，且证据不足/证据够硬由后端给（前端只展示，不自己算）。
    expect(screen.getByText(/建议主协议：Anthropic/)).toBeInTheDocument();
    expect(screen.getByText(/置信度 高/)).toBeInTheDocument();
    // 证据在结果区是短标签（完整句子留给复制文本）：同族与异族都必须能一眼区分。
    expect(screen.getAllByText("指纹同族").length).toBeGreaterThan(0);
    expect(screen.getByText("指纹异族 anthropic")).toBeInTheDocument();
  });

  it("协议检测：复制文本包含逐协议结论与总评（与屏幕同一份拼装）", async () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
    });

    const report = await copyOut();
    expect(report).toContain("测试结论：通过");
    expect(report).toContain("【Anthropic ★】");
    // 探针耗时统一用秒（一位小数）：毫秒读数在 1~20 秒量级没有分辨率价值
    expect(report).toContain("非流式 ✓ 0.8 s");
    expect(report).toContain("流式 ✓ 0.6 s");
    expect(report).toContain("【总评：建议主协议：Anthropic（置信度 高）】");
    expect(report).toContain("建议把主类型从 OpenAI 改为 Anthropic");
    // 总评必须给出「该模型该怎么配」的落地动作（一键写入的前置话术）
    expect(report).toContain("建议把该模型的协议设置为勾选 Anthropic，主格式 Anthropic");
  });

  // 一键落地：后端给了集合才给按钮，且覆盖语义必须写明（它会替换该模型已有的协议设置）。
  it("协议检测：给出可勾选集合时渲染一键按钮，回调带上关联 id、集合与主格式", async () => {
    const onApplyProtocol = vi.fn().mockResolvedValue(undefined);
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
      onApplyProtocol,
    });

    expect(screen.getByText("会替换该模型当前的协议设置")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: /一键勾选 1 条可用协议（主格式 Anthropic）/ }),
    );
    await waitFor(() =>
      expect(onApplyProtocol).toHaveBeenCalledWith(TEST_ID, ["anthropic"], "anthropic"),
    );
  });

  // 「无法区分」不再等于「什么都不给」：只要还有可用协议，就把它们全勾上、主格式沿用当前值。
  it("协议检测：无法区分但仍有可用协议时，仍给出一键勾选全部可用协议", async () => {
    const result = protocolFixture();
    const undecided = {
      ...result,
      recommendation: {
        ...result.recommendation!,
        protocols: ["openai", "anthropic"],
        primary: "openai",
      },
    };
    const onApplyProtocol = vi.fn().mockResolvedValue(undefined);
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: undecided } },
      onApplyProtocol,
    });

    fireEvent.click(
      screen.getByRole("button", { name: /一键勾选 2 条可用协议（主格式 OpenAI）/ }),
    );
    await waitFor(() =>
      expect(onApplyProtocol).toHaveBeenCalledWith(TEST_ID, ["openai", "anthropic"], "openai"),
    );
  });

  // 集合为空（一条都不可用，或可用协议全是转译路径）时不给按钮——没有可写的配置。
  it("协议检测：可勾选集合为空时不给一键按钮", () => {
    const result = protocolFixture();
    const empty = { ...result, recommendation: { ...result.recommendation!, protocols: [], primary: "" } };
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: empty } },
      onApplyProtocol: vi.fn(),
    });

    expect(screen.queryByRole("button", { name: /一键勾选/ })).toBeNull();
  });

  // 回包形状回归：协议全挂时后端可能回 protocols: null（nil 切片序列化成 null）。
  // 前端必须兜住，否则 .length 直接把整页带崩——这在「上游全挂」时真实发生过。
  it("协议检测：recommendation.protocols 为 null 时不崩", () => {
    const result = protocolFixture();
    const broken = {
      ...result,
      recommendation: { ...result.recommendation!, protocols: null },
    } as unknown as ProtocolProbeResult;
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: broken } },
      onApplyProtocol: vi.fn(),
    });

    expect(screen.queryByRole("button", { name: /一键勾选/ })).toBeNull();
    expect(screen.getAllByText(/置信度/).length).toBeGreaterThan(0);
  });

  // 继承态被覆盖是单向的：写入后供应商层再改协议，这条关联不再跟随。按钮旁必须点明。
  it("协议检测：关联处于继承态时，提示写入后会固定为该模型自己的设置", () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
      onApplyProtocol: vi.fn(),
      protocolsInherited: true,
    });

    expect(
      screen.getByText("当前为继承供应商，写入后将固定为该模型自己的设置"),
    ).toBeInTheDocument();
  });

  it("协议检测：关联已是显式覆盖时，不给继承态提示", () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
      onApplyProtocol: vi.fn(),
      protocolsInherited: false,
    });

    expect(
      screen.queryByText("当前为继承供应商，写入后将固定为该模型自己的设置"),
    ).toBeNull();
  });

  it("协议检测：未执行时复制按钮置灰", () => {
    renderDialog({ testType: "protocol", protocolTestResults: {} });

    const button = copyButton();
    expect(button.disabled).toBe(true);
    fireEvent.click(button);
    expect(copyTextWithToastMock).not.toHaveBeenCalled();
  });

  // 防「双层滚动」回归：协议矩阵曾被写死高度 + 自带 overflow，外框又固定，于是必须在
  // 小窗里滚半屏才看得全三条协议。滚动只剩外框一层（DialogBody），结果区随内容生长。
  it("结果区不再自带固定高度与滚动（避免外框之外再滚一层）", () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
    });

    // 弹窗渲染进 portal（不在 RTL 的 container 里），故按 data-slot 从 document 取。
    const area = document.querySelector('[data-slot="test-result-area"]');
    expect(area).not.toBeNull();
    expect(area?.className).not.toContain("h-64");
    expect(area?.className).not.toContain("overflow-y-auto");
  });

  // 协议检测没有模式轴：整块不渲染（不是「禁用 + 一行解释」）——既省高度，也避免让人以为漏了开关。
  it("协议检测不渲染执行模式，其余测试类型照常渲染", () => {
    const protocol = renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
    });
    expect(protocol.queryByText("执行模式")).toBeNull();
    protocol.unmount();

    renderDialog({ testType: "connectivity", testResults: resultsOf(entry({ result: capabilityResult() })) });
    expect(screen.getByText("执行模式")).toBeInTheDocument();
  });

  // 矩阵必须是「三行表格」而非三张卡：协议的读法是横向比较，卡片把同一条信息拆成三块上下堆叠。
  it("协议矩阵渲染成表格行，每行带可展开明细", () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
    });

    const rows = screen.getAllByRole("row");
    // 表头 1 行 + 三条协议各 2 行（横向比较行 + 整行证据行）
    expect(rows).toHaveLength(7);
    // summary 的文本是「▸ 明细」（箭头在后代 span 里），按正则匹配
    expect(screen.getAllByText(/明细/)).toHaveLength(3);
    // 明细默认收起但内容已在 DOM：URL 与来源是判断「测的是哪条路」的唯一依据
    expect(screen.getByText(/https:\/\/up\.example\/v1\/messages/)).toBeInTheDocument();
    expect(screen.getAllByText(/缺失字段：service_tier/).length).toBeGreaterThan(0);
    // 指纹请求的状态码也要在 DOM 里（只说「未采到」不给状态码，用户判不出是正常拒绝还是网关故障）
    expect(screen.getAllByText(/指纹请求：HTTP 404/).length).toBeGreaterThan(0);
    // 强证据必须单独可见：它是「像不像直通」的唯一正面依据，混进字段命中总数里读不出来。
    // 夹具里 anthropic 行的 strong_hit 是 ["content[].signature"]。
    expect(screen.getAllByText("强证据 1").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/不可伪造的原生特征：content\[\]\.signature/).length).toBeGreaterThan(0);
  });

  it("协议检测：证据标签是可读短句，免责说明默认收起但仍在 DOM", () => {
    renderDialog({
      testType: "protocol",
      protocolTestResults: { [TEST_ID]: { loading: false, result: protocolFixture() } },
    });

    // 「原生字段」而不是裸「字段」：后者对没看过设计的人不成词。
    expect(screen.getAllByText("原生字段 1/2").length).toBeGreaterThan(0);
    // 三句免责默认折叠，但必须仍在 DOM（复制文本也含它们，口径不能分叉）。
    expect(screen.getByText(/说明（3 条）/)).toBeInTheDocument();
    expect(screen.getByText(/耗时是单次样本，只作展示，不参与排名/)).toBeInTheDocument();
  });
});

// 协议矩阵测试夹具：一条可用（被点名）、一条部分支持（流式不合规）、一条不可用。
// 用返回注解工厂，字段拼错会被 tsc 抓住。
const protocolLeg = (over: Partial<ProtocolProbeEntry["non_stream"]> = {}): ProtocolProbeEntry["non_stream"] => ({
  status: "passed",
  passed: true,
  status_code: 200,
  duration_ms: 812,
  ...over,
});

const protocolEntryFixture = (over: Partial<ProtocolProbeEntry>): ProtocolProbeEntry => ({
  protocol: "openai",
  provider_type: "openai",
  request_url: "https://up.example/v1/chat/completions",
  url_source: "endpoint",
  endpoint_configured: true,
  verdict: "usable",
  non_stream: protocolLeg(),
  stream: protocolLeg({ duration_ms: 640 }),
  fingerprint: {
    attempted: true,
    probe_model: "llmux-protocol-probe-nonexistent",
    status_code: 404,
    family: "openai",
    same_family: true,
    native_headers: [],
  },
  fields: { hit: ["object"], miss: ["service_tier"], strong_hit: [] },
  id_prefix: "chatcmpl",
  recommended: false,
  ...over,
});

const protocolFixture = (): ProtocolProbeResult => ({
  passed: true,
  model: "gpt-4.1",
  provider: "p",
  main_protocol: "openai",
  protocols: [
    protocolEntryFixture({
      verdict: "unusable",
      non_stream: protocolLeg({ status: "failed", passed: false, status_code: 404, summary: "上游返回错误" }),
      stream: protocolLeg({ status: "failed", passed: false, status_code: 404, summary: "上游返回错误" }),
    }),
    protocolEntryFixture({
      protocol: "responses",
      provider_type: "openai-res",
      request_url: "https://up.example/v1/responses",
      url_source: "inherited",
      endpoint_configured: false,
      verdict: "partial",
      stream: protocolLeg({ status: "noncompliant", passed: false, summary: "流式响应不合规" }),
      fingerprint: {
        attempted: true,
        probe_model: "llmux-protocol-probe-nonexistent",
        status_code: 404,
        family: "anthropic",
        same_family: false,
        native_headers: [],
        message: "上游回的不是客户端错误",
      },
    }),
    protocolEntryFixture({
      protocol: "anthropic",
      provider_type: "anthropic",
      request_url: "https://up.example/v1/messages",
      url_source: "inherited",
      endpoint_configured: false,
      recommended: true,
      fingerprint: {
        attempted: true,
        probe_model: "llmux-protocol-probe-nonexistent",
        status_code: 404,
        family: "anthropic",
        same_family: true,
        native_headers: ["anthropic-ratelimit-requests-limit"],
      },
      fields: { hit: ["type", "stop_reason", "content[].signature"], miss: [], strong_hit: ["content[].signature"] },
      id_prefix: "msg",
    }),
  ],
  recommendation: {
    protocols: ["anthropic"],
    primary: "anthropic",
    confidence: "high",
    summary: "建议主协议：Anthropic",
    reason: "只有它的错误信封是自身协议形状。",
    actions: [
      "建议把主类型从 OpenAI 改为 Anthropic",
      "建议把该模型的协议设置为勾选 Anthropic，主格式 Anthropic（可一键写入，只影响这个模型）",
    ],
    notes: [
      "字段「有」才算证据，「没有」只算弱证据——很多字段是否出现取决于模型与请求",
      "耗时是单次样本，只作展示，不参与排名",
      "结论只对这个模型有效——写入的是该模型的关联设置，不影响同供应商下的其他模型",
    ],
  },
});
