package testapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/service/credprobe"
	"github.com/qkf688/llmux/service/protocolprobe"
)

// 本文件是「协议支持检测」的 HTTP 层与回包装配（GET /api/test/protocols/:id）。
//
// 结果形状与既有测试端点同族：链路解析失败走 respondResolveFailure（400/404 或
// passed=false），探测本身无论成败恒 200 + data.passed（前端要靠 passed 判断
// 「这次检测有没有产出可用结论」）。

// protocolLegResult 一条探针腿的回包。Status 取值域 = protocolprobe.LegStatus
// （passed / failed / noncompliant），noncompliant 单列的原因见该常量注释：
// 「上游明确不按流式回」与「回了 SSE 但不合规」在管理端的处置完全不同。
type protocolLegResult struct {
	Status       string `json:"status"`
	Passed       bool   `json:"passed"`
	StatusCode   int    `json:"status_code"`
	DurationMS   int64  `json:"duration_ms"`
	FirstFrameMS int64  `json:"first_frame_ms,omitempty"`
	TerminatedBy string `json:"terminated_by,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
	Summary      string `json:"summary,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

// protocolFingerprintResult 无效模型请求采到的错误信封证据。
type protocolFingerprintResult struct {
	Attempted   bool     `json:"attempted"`
	ProbeModel  string   `json:"probe_model"`
	StatusCode  int      `json:"status_code"`
	Family      string   `json:"family"`
	SameFamily  bool     `json:"same_family"`
	ErrorType   string   `json:"error_type,omitempty"`
	Message     string   `json:"message,omitempty"`
	NativeHeads []string `json:"native_headers"`
}

// protocolProbeEntry 一条协议的检测结论。字段名全部 snake_case（管理端 API 约定）。
type protocolProbeEntry struct {
	Protocol           string                    `json:"protocol"`
	ProviderType       string                    `json:"provider_type"`
	RequestURL         string                    `json:"request_url"`
	URLSource          string                    `json:"url_source"`
	EndpointConfigured bool                      `json:"endpoint_configured"`
	Verdict            string                    `json:"verdict"`
	NonStream          protocolLegResult         `json:"non_stream"`
	Stream             protocolLegResult         `json:"stream"`
	Fingerprint        protocolFingerprintResult `json:"fingerprint"`
	Fields             protocolprobe.FieldReport `json:"fields"`
	IDPrefix           string                    `json:"id_prefix"`
	// Recommended 该协议在总评里被点名（前端据此高亮那一行）。
	Recommended bool `json:"recommended"`
}

// protocolProbeResponse 一次检测的完整回包。
type protocolProbeResponse struct {
	// Passed = 至少一条协议双模式全过。单看 passed 分不出「全都不行」与
	// 「有行部分支持」，细节看 protocols 数组。
	Passed         bool                         `json:"passed"`
	GroupID        uint                         `json:"group_id"`
	GroupName      string                       `json:"group_name"`
	Provider       string                       `json:"provider"`
	Model          string                       `json:"model"`
	MainProtocol   string                       `json:"main_protocol"`
	Protocols      []protocolProbeEntry         `json:"protocols"`
	Recommendation protocolprobe.Recommendation `json:"recommendation"`
}

// TestProtocolsHandler 协议支持检测（GET /api/test/protocols/:id，id = ModelWithProvider 主键）。
//
// 只读：不做任何配置写入。本端点回答的是「同一个模型在这家上游支持哪几种协议、
// 哪条路更像直通」，不是「该不该开这个端点」——后者见 next-do #1（按模型覆盖协议端点）。
func TestProtocolsHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}
	ctx := c.Request.Context()
	set, err := ResolveProtocolTargets(ctx, id, c.Request.Header)
	if err != nil {
		respondResolveFailure(c, err, "ModelWithProvider not found")
		return
	}

	states := protocolprobe.Run(ctx, set.Targets, set.Header, set.Model)
	entries, inputs := buildProtocolEntries(states)
	rec := protocolprobe.Recommend(set.MainProtocol, inputs)

	passed := false
	for i := range entries {
		if entries[i].Verdict == string(protocolprobe.VerdictUsable) {
			passed = true
			break
		}
	}
	for i := range entries {
		entries[i].Recommended = rec.Protocol != "" && entries[i].Protocol == rec.Protocol
	}

	httpresp.Success(c, protocolProbeResponse{
		Passed:         passed,
		GroupID:        set.Group.ID,
		GroupName:      set.Group.Name,
		Provider:       set.ProviderName,
		Model:          set.Model,
		MainProtocol:   string(set.MainProtocol),
		Protocols:      entries,
		Recommendation: rec,
	})
}

// buildProtocolEntries 把原始探测产物折算成回包条目与总评输入（同一份折算，
// 两处不各算一次——条目与总评一旦分叉，面板显示的和建议依据的就对不上）。
func buildProtocolEntries(states []protocolprobe.RawResult) ([]protocolProbeEntry, []protocolprobe.Input) {
	entries := make([]protocolProbeEntry, 0, len(states))
	inputs := make([]protocolprobe.Input, 0, len(states))
	for _, state := range states {
		target := state.Target

		fields := protocolprobe.FieldReport{Hit: []string{}, Miss: []string{}}
		idPrefix := ""
		if state.NonStream.Passed {
			fields = protocolprobe.InspectFields(target.Format, state.NonStream.Body)
			idPrefix = protocolprobe.IDPrefix(state.NonStream.Body)
		}

		// 原生响应头两条响应都看（成功体与错误体）：网关有时只在错误路径透传上游头。
		heads := mergeNativeHeaders(target.Protocol, state.NonStream.Headers, state.Fingerprint.Headers)

		family := protocolprobe.FingerprintFamily(state.Fingerprint)
		fingerprint := protocolFingerprintResult{
			Attempted:   true,
			ProbeModel:  protocolprobe.ProbeModelName,
			StatusCode:  state.Fingerprint.StatusCode,
			Family:      string(family),
			SameFamily:  family != protocolprobe.FamilyUnknown && family == protocolprobe.FamilyOfProtocol(target.Protocol),
			NativeHeads: heads,
		}
		switch {
		case family != protocolprobe.FamilyUnknown:
			_, errType, message := protocolprobe.ClassifyError(state.Fingerprint.Body)
			fingerprint.ErrorType = errType
			fingerprint.Message = truncateRunes(message, fingerprintMessageRunes)
		case state.Fingerprint.Passed:
			fingerprint.Message = "上游没有拒绝这个不存在的模型名，采不到错误信封"
		case state.Fingerprint.StatusCode < 400 || state.Fingerprint.StatusCode >= 500:
			fingerprint.Message = "上游回的不是客户端错误（" + state.Fingerprint.Summary + "），错误信封不作证据"
		}

		nonStreamLeg := buildNonStreamLeg(state.NonStream)
		streamLeg := buildStreamLeg(state.Stream, target.Format)

		input := protocolprobe.Input{
			Protocol:           target.Protocol,
			NonStream:          protocolprobe.LegStatus(nonStreamLeg.Status),
			Stream:             protocolprobe.LegStatus(streamLeg.Status),
			Family:             family,
			NativeHeaders:      heads,
			Fields:             fields,
			NonStreamMS:        nonStreamLeg.DurationMS,
			EndpointConfigured: target.EndpointConfigured,
		}
		inputs = append(inputs, input)
		entries = append(entries, protocolProbeEntry{
			Protocol:           string(target.Protocol),
			ProviderType:       target.ProviderType,
			RequestURL:         target.RequestURL,
			URLSource:          target.URLSource,
			EndpointConfigured: target.EndpointConfigured,
			Verdict:            string(protocolprobe.VerdictOf(input)),
			NonStream:          nonStreamLeg,
			Stream:             streamLeg,
			Fingerprint:        fingerprint,
			Fields:             fields,
			IDPrefix:           idPrefix,
		})
	}
	return entries, inputs
}

func buildNonStreamLeg(out credprobe.CompletionProbeResult) protocolLegResult {
	status := protocolprobe.LegFailed
	if out.Passed {
		status = protocolprobe.LegPassed
	}
	return protocolLegResult{
		Status:     string(status),
		Passed:     out.Passed,
		StatusCode: out.StatusCode,
		DurationMS: out.DurationMS,
		ErrorType:  out.ErrorType,
		Summary:    out.Summary,
		Detail:     out.Detail,
	}
}

// buildStreamLeg 把流式探测折成三态：合规=passed、明确不按流式回/连不上=failed、
// 回了 SSE 但缺终止事件或无内容增量=noncompliant。
//
// 折叠与严格门在这里就地调用（foldStream / validateStreamFold），与既有四个测试端点
// 共用同一份实现——通过标准只允许有一份。
func buildStreamLeg(probe credprobe.StreamProbeResult, format consts.WireFormat) protocolLegResult {
	leg := protocolLegResult{
		StatusCode:   probe.StatusCode,
		DurationMS:   probe.DurationMS,
		FirstFrameMS: probe.FirstFrameMS,
		ErrorType:    probe.ErrorType,
		Summary:      probe.Summary,
		Detail:       probe.Detail,
	}
	// 没进到分帧阶段就没有 terminated_by / 合规可言（内容类型不对、连接失败都属此列）。
	if !probe.Passed || !probe.Framed {
		leg.Status = string(protocolprobe.LegFailed)
		return leg
	}
	fold := foldStream(format, probe.Frames)
	leg.TerminatedBy = fold.terminatedBy
	if err := validateStreamFold(fold); err != nil {
		leg.Status = string(protocolprobe.LegNonCompliant)
		leg.ErrorType = streamFailureType(err)
		leg.Summary = "流式响应不合规"
		leg.Detail = err.Error()
		return leg
	}
	leg.Status = string(protocolprobe.LegPassed)
	leg.Passed = true
	return leg
}

// fingerprintMessageRunes 错误摘要回显上限（按 rune 切，避免把中文劈成半个字符）。
const fingerprintMessageRunes = 300

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// mergeNativeHeaders 合并两条响应里的本家族原生头并去重（同一份判定只有一处口径）。
func mergeNativeHeaders(protocol consts.Protocol, headers ...http.Header) []string {
	family := protocolprobe.FamilyOfProtocol(protocol)
	seen := map[string]bool{}
	var out []string
	for _, header := range headers {
		for _, name := range protocolprobe.NativeHeaders(header, family) {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	if out == nil {
		return []string{}
	}
	// NativeHeaders 内已按字典序返回，逐个追加的结果仍是稳定序。
	return out
}
