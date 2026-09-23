package testapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/credprobe"
)

// failFunc 一次测试失败的统一回包（HTTP 200 + passed:false）。rawOutput 可选：
// 只有真的拿到上游原文时才带键，与「键缺失 = 没拿到」的既有口径一致。
type failFunc = func(errorType, summary, detail string, rawOutput ...string)

// probeOutcome 一次探针的可判定产物。两种模式共用同一形状，因此端点侧不需要为流式
// 分叉一套编排——它们只看 body / rawText，与改造前完全同构。
type probeOutcome struct {
	stream bool
	// format 是上游响应体形状（由链路的注册类型派生，与 chat 主链路同源）。
	// 放在这里而不是让每个端点自己 WireFormatOf：四个端点各算一次必漂移。
	format  consts.WireFormat
	body    []byte // 判定输入：非流式 = 上游响应体；流式 = SSE 折叠出的非流式形状
	rawText string // 回显原文：非流式 = 上游响应体；流式 = SSE 文本（可能被头尾截断）
	fold    streamFold
	probe   credprobe.StreamProbeResult
}

// modeFields 回包里的「跑的是哪种模式」与流式诊断键。成功与失败两条路径都写，
// 因为「流式失败」这件事本身就是要给用户看的事实。
func (o probeOutcome) modeFields() map[string]any {
	fields := map[string]any{"mode": testModeOf(o.stream)}
	// 流式统计只在真的进到分帧阶段后才有信息量（探针的 Framed）：在 Content-Type 门或
	// 连接阶段就失败时 fold 是零值，terminated_by 的空串会被误读成「上游没发终止帧」
	// 这一真实缺陷，所以整个 stream 键缺席而不是回一个空对象。
	if o.stream && o.probe.Framed {
		fields["stream"] = streamStatsPayload(o.fold, o.probe)
	}
	return fields
}

// applyOutcomeFields 把模式与流式诊断并进回包（成功侧由 passedCapabilityResult /
// 连通性成功分支调用；失败侧由 fail 闭包调用，两侧同源）。
func applyOutcomeFields(result map[string]any, outcome probeOutcome) {
	for key, value := range outcome.modeFields() {
		result[key] = value
	}
}

// passedCapabilityResult 装配 capability 类测试的成功回包。四个端点此前各抄一遍
// 「message + provider/model/provider_type + raw_output + mode 键」，第五类测试就要抄第五份。
// extraKeys 传该测试自己的提取结果键（thinking_text / vision_text / parsed / payload 文本）。
func passedCapabilityResult(link *testLink, outcome probeOutcome, message string, extraKeys map[string]any) map[string]any {
	result := passedTestResult(link)
	result["message"] = message
	result["provider"] = link.ProviderName
	result["model"] = link.Model
	result["provider_type"] = link.Type
	result["raw_output"] = outcome.rawText
	applyOutcomeFields(result, outcome)
	for key, value := range extraKeys {
		result[key] = value
	}
	return result
}

// runCompletionTestFlow 是所有 completion 型测试端点共享的公共流程：
// 解析关联链路 → 取测试体 → 按模式探针上游（非流式 RunCompletionProbe / 流式 RunStreamProbe）
// → 流式时折叠并过严格门 → 任一失败按 testapi 结果形状回包并返回 ok=false（调用方直接 return）。
//
// 探针通过后把链路与**可判定产物**交回，由各端点做领域判定（提取逻辑与通过标准是端点间
// 唯一的差异点）。流式模式下 body 已是折叠出的非流式形状，所以 extractor 一行都不用改——
// 通过标准只有一份是设计约束，不是巧合（见 stream_shape.go 头注释）。
//
// 流式的严格门在这里统一执行：Content-Type 合规（探针内）+ 有内容增量 + 有终止事件
// （validateStreamFold）。放在这里而不是端点里，是因为四个端点的要求完全一致，
// 抄四遍必然出现某一处漏一门。
//
// 供应商级模型测试（POST /providers/:id/test）不走本流：它的回包形状与链路解析都不同
// （显式 group_id、按 (model, group) 归位），且本次不提供流式模式。
func runCompletionTestFlow(c *gin.Context, name string, buildBody func(providerType string) ([]byte, error), stream bool) (link *testLink, outcome probeOutcome, fail failFunc, ok bool) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return nil, outcome, nil, false
	}

	link, err := ResolveAssociationLink(ctx, id)
	if err != nil {
		respondResolveFailure(c, err, "ModelWithProvider not found")
		return nil, outcome, nil, false
	}
	outcome.stream = stream
	format, formatKnown := providers.WireFormatOf(link.Type)
	outcome.format = format

	slog.Info("Testing "+name,
		"mode", testModeOf(stream),
		"proxy", link.Client.GetProxy(),
		"provider", link.ProviderName,
		"model", link.Model,
		"type", link.Type,
		"group", link.Group.Name)

	// 闭包捕获命名返回值 outcome：失败发生得越早，modeFields 越接近「只有 mode 一个键」，
	// 这正是想要的语义（没跑到的阶段不编造诊断）。
	fail = func(errorType, summary, detail string, rawOutput ...string) {
		payload := failedTestResult(link, errorType, summary, detail)
		if len(rawOutput) > 0 {
			payload["raw_output"] = rawOutput[0]
		}
		for key, value := range outcome.modeFields() {
			payload[key] = value
		}
		httpresp.Success(c, payload)
	}

	if !formatKnown {
		// 未声明响应体形状时**不兜底成 OpenAI 形状**：兜底会让一个非 OpenAI 协议的上游
		// 被按错误形状提取，报成「模型未返回内容」——与 providers.WireFormatOf 刻意不兜底同因。
		fail(common.ErrorTypeProvider, "不支持的提供商类型",
			"供应商类型 "+link.Type+" 未声明上游响应体形状（WireFormat），无法判定测试响应")
		return nil, outcome, nil, false
	}

	testBody, err := buildBody(link.Type)
	if err != nil {
		fail(common.ErrorTypeValidation, "不支持的提供商类型", err.Error())
		return nil, outcome, nil, false
	}

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)

	if stream {
		if !runStreamLeg(c, link, &outcome, fail, header, testBody) {
			return nil, outcome, nil, false
		}
		return link, outcome, fail, true
	}

	out := credprobe.RunCompletionProbe(credprobe.CompletionProbeRequest{
		Ctx:     ctx,
		Client:  link.Client,
		Timeout: common.DiagnosticConnectivityTestTimeout,
		Headers: header,
		Model:   link.Model,
		Body:    testBody,
	})
	if !out.Passed {
		// raw_output 口径：拿到上游响应且成功读完（含空 body、未通过判定）才带键；
		// BuildReq/连接/读体失败无原文，不带键。
		if out.Summary == credprobe.ProbeSummaryUpstreamError {
			fail(out.ErrorType, out.Summary, out.Detail, string(out.Body))
		} else {
			fail(out.ErrorType, out.Summary, out.Detail)
		}
		return nil, outcome, nil, false
	}
	outcome.body = out.Body
	outcome.rawText = string(out.Body)
	return link, outcome, fail, true
}

// runStreamLeg 流式模式的一段腿：探针 → 折叠 → 严格门；返回是否可交回端点做领域判定。
// 与 runCompletionTestFlow 同包同职责，只是把「模式分叉」收进一个函数，公共流本体保持线性可读。
//
// outcome 必须是指针：fail 闭包捕获的是调用方的那个变量，这里若按值改，折叠结果对闭包
// 不可见，失败回包就会丢掉整个 stream 诊断块（「流被切断」和「不支持流式」变成同一条结论）。
func runStreamLeg(c *gin.Context, link *testLink, outcome *probeOutcome, fail failFunc, header http.Header, testBody []byte) bool {
	probe := credprobe.RunStreamProbe(credprobe.StreamProbeRequest{
		Ctx:     c.Request.Context(),
		Client:  link.Client,
		Timeout: common.DiagnosticConnectivityTestTimeout,
		Headers: header,
		Model:   link.Model,
		Body:    testBody,
	})
	outcome.probe = probe

	// 只要真的进到分帧阶段就先折叠：探针中途失败（滴流超期、超预算）时，已收到的帧
	// 仍然要变成 terminated_by / frames 报出去，否则用户分不清「一条都没收到」与
	// 「收了 800 帧后被切断」——这两件事的处置方式完全不同。
	if probe.Framed {
		outcome.fold = foldStream(outcome.format, probe.Frames)
		outcome.rawText = probe.RawOutput
		outcome.body = outcome.fold.canonical
	}

	if !probe.Passed {
		// raw_output 口径与非流式一致：探针交回了上游原文就带键；上游未通过判定即使回了空体
		// 也带键（空串），让「上游答了但答得空」与「压根没拿到响应」在回包里可区分。
		if probe.RawOutput != "" || probe.Summary == credprobe.ProbeSummaryUpstreamError {
			fail(probe.ErrorType, probe.Summary, probe.Detail, probe.RawOutput)
		} else {
			fail(probe.ErrorType, probe.Summary, probe.Detail)
		}
		return false
	}

	if err := validateStreamFold(outcome.fold); err != nil {
		fail(streamFailureType(err), "流式响应校验失败", err.Error(), probe.RawOutput)
		return false
	}
	return true
}
