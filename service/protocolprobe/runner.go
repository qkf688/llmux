package protocolprobe

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/credprobe"
	"github.com/qkf688/llmux/service/transform"
)

// 本文件是协议探测的**编排层**：按两段并发把三条协议各打三条探针，把原始产物交回调用方。
//
// 为什么并发放在 service 而不放在 handler：handler 层有一条机械门禁「零裸 go 语句」
// （会写库的后台 goroutine 必须走 bgtask 登记，handler/v1/chat_bgtask_gate_test.go），
// 而「一次检测并发打 9 条请求」正是编排职责、本就该在 service；handler 只该做 HTTP 与
// 协议语义判定（流式折叠 / 回包组装）。
//
// 为什么两段而非一次全并发：① 第二段（流式）无论第一段结果如何都要跑——「只有流式能用」
// 正是要发现的情况；② 一次并发 9 条会让限流严格的上游回 429，而 429 会被读成「协议不可用」。
// 每条探针各带 common.DiagnosticProtocolProbeTimeout 预算，故整段最坏约一分钟。

// ProbeModelName 指纹探针用的模型名：必须不可能命中真实模型，否则这条探针会真的跑一次
// 推理（花钱且污染耗时）。带 llmux 前缀便于在上游日志里认出是诊断流量。
const ProbeModelName = "llmux-protocol-probe-nonexistent"

// Target 一条待探测的协议路径：客户端已装配好凭据，请求 URL 与测试体已定。
type Target struct {
	Protocol           consts.Protocol
	ProviderType       string
	Format             consts.WireFormat
	Body               []byte
	RequestURL         string
	URLSource          string
	EndpointConfigured bool
	Client             providers.Provider
}

// RawResult 一条协议的探测原始产物。流式的折叠与合规判定**不在本包**：那是协议语义，
// 属 handler/testapi（与既有四个测试端点同一份门），本包只负责把流打回来。
type RawResult struct {
	Target      Target
	NonStream   credprobe.CompletionProbeResult
	Fingerprint credprobe.CompletionProbeResult
	Stream      credprobe.StreamProbeResult
}

// Run 跑完一次检测，返回与 targets 同序同长的结果数组。
func Run(ctx context.Context, targets []Target, header http.Header, model string) []RawResult {
	results := make([]RawResult, len(targets))
	for i := range targets {
		results[i] = RawResult{Target: targets[i]}
	}

	// 第一段：非流式 + 无效模型指纹，各协议内部两条并行 → 峰值 6 条并发。
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			results[idx].NonStream = runNonStream(ctx, targets[idx], header, model)
		}(i)
		go func(idx int) {
			defer wg.Done()
			results[idx].Fingerprint = runFingerprint(ctx, targets[idx], header)
		}(i)
	}
	wg.Wait()

	// 第二段：流式，三条并发。
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx].Stream = credprobe.RunStreamProbe(credprobe.StreamProbeRequest{
				Ctx:     ctx,
				Client:  targets[idx].Client,
				Timeout: common.DiagnosticProtocolProbeTimeout,
				Headers: header,
				Model:   model,
				Body:    targets[idx].Body,
			})
		}(i)
	}
	wg.Wait()

	for i := range results {
		slog.Info("协议探测完成",
			"protocol", results[i].Target.Protocol,
			"provider_type", results[i].Target.ProviderType,
			"url", results[i].Target.RequestURL,
			"non_stream", results[i].NonStream.Passed,
			"non_stream_status", results[i].NonStream.StatusCode,
			"stream", results[i].Stream.Passed,
			"stream_status", results[i].Stream.StatusCode,
			"fingerprint_status", results[i].Fingerprint.StatusCode,
			"fingerprint_family", string(FingerprintFamily(results[i].Fingerprint)))
	}
	return results
}

// runNonStream 打一条非流式 completion。判定 = 探针通过（credprobe 走
// common.IsPassingStatus）+ 响应能按本协议形状解析（transform.ValidateResponseShape，
// 检测类内容判定的唯一入口）。形状不对就是这条路不可用，故直接改判。
func runNonStream(ctx context.Context, target Target, header http.Header, model string) credprobe.CompletionProbeResult {
	out := credprobe.RunCompletionProbe(credprobe.CompletionProbeRequest{
		Ctx:     ctx,
		Client:  target.Client,
		Timeout: common.DiagnosticProtocolProbeTimeout,
		Headers: header,
		Model:   model,
		Body:    target.Body,
	})
	if !out.Passed {
		return out
	}
	if err := transform.ValidateResponseShape(target.Format, out.Body); err != nil {
		out.Passed = false
		out.ErrorType = common.ErrorTypeValidation
		out.Summary = "响应形状校验失败"
		out.Detail = err.Error()
	}
	return out
}

// runFingerprint 打一条「模型名不存在」的请求，采上游的错误信封形状。走同一条协议路径、
// 同一个客户端，只把模型名换掉（BuildReq 以入参 model 覆写请求体里的 model 字段），
// 故无需为它单独注册测试体。
func runFingerprint(ctx context.Context, target Target, header http.Header) credprobe.CompletionProbeResult {
	return credprobe.RunCompletionProbe(credprobe.CompletionProbeRequest{
		Ctx:     ctx,
		Client:  target.Client,
		Timeout: common.DiagnosticProtocolProbeTimeout,
		Headers: header,
		Model:   ProbeModelName,
		Body:    target.Body,
	})
}

// FingerprintFamily 只在「客户端错误」时采信错误信封：401/403/5xx 的错误体是网关自己的
// 鉴权/故障信封，拿它判「这条协议是不是被转译」是拿噪声当证据。
func FingerprintFamily(out credprobe.CompletionProbeResult) Family {
	if out.Passed || out.StatusCode < 400 || out.StatusCode >= 500 {
		return FamilyUnknown
	}
	family, _, _ := ClassifyError(out.Body)
	return family
}
