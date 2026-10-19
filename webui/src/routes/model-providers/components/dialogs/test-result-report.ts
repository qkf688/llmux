/**
 * 模型测试结果区的「段落定义」与「复制整份结果」单一来源（纯函数，无 IO）。
 *
 * 为什么段名和取值放在这里：结果区屏幕上显示的段落与复制出去的文本必须是同一份东西。
 * 两边各写一份时，屏幕改了排版、剪贴板还留着旧格式这类漂移没有任何检查会红——所以
 * 段名、流式诊断四项数、各段取值都只在这里定一次，展示（LogBlock / StreamStatsSection）
 * 与复制（build*Report）都从这里取。
 */
import type { ModelProviderTestResult, ProtocolProbeEntry, ProtocolProbeResult } from "@/lib/api";
import { protocolLabel } from "@/lib/protocols";
import type { ReactTestResultState } from "@/stores/model-providers/types";
import type { TestType } from "../../types";

/** 走「能力测试」形状（按关联+模式存一格 + 结论 + 可选类型专属段）的测试类型。
 *  ReAct 自持一条 SSE 流、协议检测回一张协议矩阵，两者结果形状都不同，均不在此列。 */
export type CapabilityTestType = Exclude<TestType, "react" | "protocol">;

/** 一类能力测试的展示 + 复制描述。段名与取值为单一来源，UI 与剪贴板共用；
 *  新增能力测试类型只需在调用方那张表里加一行，本模块不动（OCP）。 */
export type CapabilityTestSpec = {
  /** 通过且后端没给 message 时的兜底展示语（仅上屏，不进复制文本） */
  passedLabel: string;
  /** 失败且后端没给 error/message 时的兜底展示语（仅上屏，不进复制文本） */
  failedLabel: string;
  /** 类型专属段（结构化输出的解析结果 / thinking 的推理内容 / vision 的识别结果）。
   *  取值返回空则该段既不上屏也不进复制文本。 */
  detail?: {
    label: string;
    value: (result: ModelProviderTestResult) => string | null;
  };
};

// 终止事件值域（镜像后端 handler/testapi 的 streamTerminal*）。未知值原样展示，
// 后端加新值时不至于在这里静默显示成空。
const TERMINAL_EVENT_LABELS: Record<string, string> = {
  "[DONE]": "[DONE]",
  message_stop: "message_stop",
  "response.completed": "response.completed",
  "response.failed": "response.failed",
  "response.incomplete": "response.incomplete",
  none: "未收到（流被切断）",
};

/** 流式诊断四项（帧数 / 首帧耗时 / 总耗时 / 终止事件）：判断「渠道是不是真支持流式」
 *  的直接证据（缺终止事件 = 流被切断，是最常见也最难从一句「失败」里看出来的缺陷）。
 *  非流式或上游连 SSE 都没起时后端不下发，返空数组。 */
export function streamStatsItems(result: ModelProviderTestResult): { label: string; value: string }[] {
  const stats = result.stream;
  if (!stats) return [];
  return [
    { label: "SSE 帧数", value: String(stats.frames) },
    // 一帧都没收到时首帧耗时是 0，与「瞬时首帧」看起来一样——显示成占位符，
    // 免得把「上游压根没吐字」读成「响应飞快」。
    { label: "首帧耗时", value: stats.frames > 0 ? `${stats.first_frame_ms} ms` : "—" },
    { label: "总耗时", value: `${stats.duration_ms} ms` },
    { label: "终止事件", value: TERMINAL_EVENT_LABELS[stats.terminated_by] ?? stats.terminated_by },
  ];
}

/** 上游原始输出的段名随模式变（流式收的是 SSE 折叠文本），展示与复制共用同一判定。 */
export function rawOutputLabel(result: ModelProviderTestResult): string {
  return result.mode === "stream" ? "原始 SSE 输出" : "原始输出";
}

// 复制文本的段名统一用【】包住：粘到聊天窗口/工单里自带分隔，不依赖 Markdown 渲染。
const PASS_HEAD = "测试结论：通过";
const FAIL_HEAD = "测试结论：失败";

function joinSections(head: string, sections: { label: string; body: string }[]): string {
  return [head, ...sections.map((section) => `【${section.label}】\n${section.body}`)].join("\n\n");
}

/** 能力测试（连通性 / 结构化输出 / thinking / vision）的整份复制文本：
 *  结论 → 流式诊断 → 类型专属段 → 上游原始输出。空段不进文本。 */
export function buildCapabilityReport(spec: CapabilityTestSpec, result: ModelProviderTestResult): string {
  const passed = result.passed === true;
  // 原因原文照抄后端 error/message，不做裁剪：这份文本的用途就是发出去排查。
  // 通过时后端也可能给 message（如「连通性正常」），标成「说明」避免与失败原因混读。
  const reason = result.error || result.message || "";
  const reasonLine = reason ? `${passed ? "说明" : "失败原因"}：${reason}` : "";
  const head = [passed ? PASS_HEAD : FAIL_HEAD, reasonLine].filter((line) => line !== "").join("\n");
  return joinSections(head, capabilitySections(spec, result));
}

function capabilitySections(
  spec: CapabilityTestSpec,
  result: ModelProviderTestResult
): { label: string; body: string }[] {
  const sections: { label: string; body: string }[] = [];
  const stats = streamStatsItems(result);
  if (stats.length > 0) {
    sections.push({ label: "流式诊断", body: stats.map((item) => `${item.label}：${item.value}`).join("\n") });
  }
  const detail = spec.detail;
  const detailValue = detail?.value(result) ?? null;
  if (detail && detailValue) {
    sections.push({ label: detail.label, body: detailValue });
  }
  const rawOutput = result.raw_output ?? "";
  if (rawOutput) {
    sections.push({ label: rawOutputLabel(result), body: rawOutput });
  }
  return sections;
}

/** ReAct 测试的整份复制文本：结论 → 半截/完整日志。
 *  返回 null = 本状态没有可复制的结论（未执行 / 正在跑 / 已取消且未出结论），
 *  调用方据此置灰按钮——与结果区「已取消」独立成态、不显示成失败的口径一致。 */
export function buildReactReport(state: ReactTestResultState): string | null {
  if (state.success === null && !state.error) return null;
  const passed = state.success === true && !state.error;
  const reasonLine = state.error ? `失败原因：${state.error}` : "";
  const head = [passed ? PASS_HEAD : FAIL_HEAD, reasonLine].filter((line) => line !== "").join("\n");
  const log = state.messages.trim();
  return log ? joinSections(head, [{ label: "测试日志", body: log }]) : head;
}

/* ---------- 协议支持检测 ---------- */

/** 协议可用性判定的人读名。noncompliant 刻意与 failed 分开：前者是「实现了但不合规」
 *  （缺终止事件/无内容增量），后者是「压根不按流式回」，处置完全不同。 */
const PROTOCOL_VERDICT_LABEL: Record<string, string> = {
  usable: "可用",
  partial: "部分支持",
  unusable: "不可用",
};

const PROTOCOL_CONFIDENCE_LABEL: Record<string, string> = {
  high: "高",
  medium: "中",
  low: "低",
};

/** 判定/置信度的人读名：结果区与复制文本共用同一份（两边各写一份必漂移）。 */
export function protocolVerdictLabel(verdict: string): string {
  return PROTOCOL_VERDICT_LABEL[verdict] ?? verdict;
}

export function protocolConfidenceLabel(confidence: string): string {
  return PROTOCOL_CONFIDENCE_LABEL[confidence] ?? confidence;
}

/** 协议名（推荐的那条加星号标记）——展示与复制共用。 */
export function protocolName(entry: ProtocolProbeEntry): string {
  const name = protocolLabel(entry.protocol);
  return entry.recommended ? `${name} ★` : name;
}

/** 单条协议的一行摘要：判定 + 非流式/流式耗时（失败时带原因短语）。展示与复制共用，
 *  否则屏幕上写「流式不合规」、剪贴板里只剩「failed」这类漂移没有任何检查会红。 */
/** 探针耗时统一显示为秒（保留一位小数）。协议探针动辄 1~20 秒，毫秒读数在这个量级没有
 *  分辨率价值，反而让三列数字更难横向比较；`0.9 s` / `21.9 s` 一眼就能比。 */
export function formatDurationSeconds(durationMs: number): string {
  return `${(durationMs / 1000).toFixed(1)} s`;
}

/** 单条协议的一行摘要：判定 + 非流式/流式耗时（失败时带原因短语）。展示与复制共用，
 *  否则屏幕上写「流式不合规」、剪贴板里只剩「failed」这类漂移没有任何检查会红。 */
export function protocolSummaryLine(entry: ProtocolProbeEntry): string {
  const leg = (label: string, result: ProtocolProbeEntry["non_stream"]) => {
    const mark = result.status === "passed" ? "✓" : result.status === "noncompliant" ? "⚠" : "✗";
    const timing =
      result.status === "passed"
        ? formatDurationSeconds(result.duration_ms)
        : result.summary || result.detail || result.error_type || "未通过";
    return `${label} ${mark} ${timing}`;
  };
  const verdict = PROTOCOL_VERDICT_LABEL[entry.verdict] ?? entry.verdict;
  return [verdict, leg("非流式", entry.non_stream), leg("流式", entry.stream)].join(" · ");
}

/** 证据摘要：指纹同族 / 原生响应头 / 原生字段命中。展示与复制共用。 */
export function protocolEvidenceLine(entry: ProtocolProbeEntry): string {
  const parts: string[] = [];
  if (entry.fingerprint.same_family) {
    parts.push(`错误信封与自身协议同族（${entry.fingerprint.family}）`);
  } else if (entry.fingerprint.message) {
    parts.push(entry.fingerprint.message);
  }
  if (entry.fingerprint.native_headers.length > 0) {
    parts.push(`原生响应头 ${entry.fingerprint.native_headers.join(", ")}`);
  }
  if (entry.id_prefix) {
    parts.push(`响应 id 前缀 ${entry.id_prefix}`);
  }
  const total = entry.fields.hit.length + entry.fields.miss.length;
  if (total > 0) {
    const miss = entry.fields.miss.length > 0 ? `（缺 ${entry.fields.miss.join(", ")}）` : "";
    parts.push(`原生字段 ${entry.fields.hit.length}/${total} 命中${miss}`);
  }
  return parts.join(" · ");
}

/** 协议证据的**短标签**（结果区表格用）：指纹结论 / 字段命中 / 原生头数。
 *  为什么不能让表格直接渲染 protocolEvidenceLine：那是给复制文本的完整句子（含缺失清单），
 *  在窄弹窗里会挤成一字一行。两处都放在本模块，屏幕与剪贴板仍只有这一份语义定义。 */
export function protocolEvidenceChips(entry: ProtocolProbeEntry): string[] {
  const chips: string[] = [];
  if (entry.fingerprint.same_family) {
    chips.push("指纹同族");
  } else if (entry.fingerprint.family === "unknown") {
    chips.push("指纹未采到");
  } else {
    chips.push(`指纹异族 ${entry.fingerprint.family}`);
  }
  const total = entry.fields.hit.length + entry.fields.miss.length;
  if (total > 0) {
    chips.push(`原生字段 ${entry.fields.hit.length}/${total}`);
  }
  if (entry.fingerprint.native_headers.length > 0) {
    chips.push(`原生头 ${entry.fingerprint.native_headers.length}`);
  }
  return chips;
}

/** 协议检测的整份复制文本：结论 → 逐协议（判定/耗时/地址/证据）→ 总评。
 *  返回 null = 本状态没有可复制的结论（未执行 / 执行中 / 已取消）。 */
export function buildProtocolReport(result: ProtocolProbeResult | null): string | null {
  if (!result) return null;
  const recommendation = result.recommendation;
  const passed = result.passed === true;
  const head = [
    passed ? PASS_HEAD : FAIL_HEAD,
    `模型：${result.model ?? "—"}${result.provider ? `（供应商 ${result.provider}）` : ""}`,
  ].join("\n");
  const sections: { label: string; body: string }[] = (result.protocols ?? []).map((entry) => ({
    label: protocolName(entry),
    body: [
      protocolSummaryLine(entry),
      `地址：${entry.request_url}（${entry.url_source === "endpoint" ? "已配置端点" : "继承 Base URL"}）`,
      protocolEvidenceLine(entry),
    ]
      .filter((line) => line !== "")
      .join("\n"),
  }));
  if (recommendation) {
    const confidence = protocolConfidenceLabel(recommendation.confidence);
    sections.push({
      label: `总评：${recommendation.summary}（置信度 ${confidence}）`,
      body: [
        recommendation.reason,
        ...recommendation.actions.map((action) => `· ${action}`),
        ...recommendation.notes.map((note) => `· ${note}`),
      ]
        .filter((line) => line !== "")
        .join("\n"),
    });
  }
  return joinSections(head, sections);
}
