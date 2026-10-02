package chat

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/qkf688/llmux/models"
)

// BalanceInput 是 BalanceChat 的入参集合。
type BalanceInput struct {
	Start             time.Time
	Style             string
	Before            Before
	ProvidersWithMeta ProvidersWithMeta
	ReqMeta           models.ReqMeta
}

// BalanceResult 是转发成功后的产物：待回写给客户端的响应，以及后处理落库
// （RecordLog）所需的关联信息。
type BalanceResult struct {
	Response     *http.Response
	LogID        uint
	ProviderName string

	// SideChannel 承载转换旁路产物：上游原始 SSE 累积体与上游原始 usage。
	// 直通路径无转换层可旁路，故为 nil。
	SideChannel *models.TransformSideChannel

	// ServingModelWithProviderID 本次响应实际由哪个关联提供（含故障转移后的赢家）；
	// 0 = 无可比对对象。提交后失败发生在响应已交付之后、下游只能经日志管道感知，日志
	// 侧据此判断「这段对话的钉定是否还指着刚失败的这家」——详见 clearStickyTarget。
	ServingModelWithProviderID uint
}

// BalanceChat 是两条候选路径（真实 / 虚拟模型）的公共出口。超时归因映射只在本
// 函数返回处做一次（Stage 33 收口）：网关掐断（deadline）统一 504、调用方撤走
// 原样透传——内层任何出口禁止自映射，否则漏一处就有一条路径回裸 500。
func BalanceChat(ctx context.Context, in BalanceInput) (*BalanceResult, error) {
	res, err := balanceChat(ctx, in)
	return res, mapAbortErr(err)
}

func balanceChat(ctx context.Context, in BalanceInput) (*BalanceResult, error) {
	slog.Info("request",
		"model", in.Before.Model,
		"stream", in.Before.Stream,
		"tool_call", in.Before.toolCall,
		"structured_output", in.Before.structuredOutput,
		"image", in.Before.image,
	)

	// 检查是否是虚拟模型
	if in.ProvidersWithMeta.IsVirtualModel {
		return balanceChatVirtual(ctx, in)
	}

	pwm := in.ProvidersWithMeta

	// 超时/重试已全局化（per-model time_out/max_retry 移除）：
	// 单次尝试窗口 = 按入站 stream 分流的全局值（流式等响应头 / 非流式等整段回答，
	// Stage 38）；总时长上限 = 请求 ctx 的 deadline（Stage 33 起由 handler 入口经
	// WithRequestTotalTimeout 挂载，到点连流式输出一起掐断）。
	// 二者解耦语义不变：窗口到点只消耗该窗口，总上限未耗尽时候选 2 仍有完整窗口
	//（原实现同一值双角色）。
	window := resolveUpstreamWindow(ctx, in.Before.Stream)

	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:            ctx,
		Start:          in.Start,
		Style:          in.Style,
		Before:         in.Before,
		ReqMeta:        in.ReqMeta,
		IOLog:          pwm.IOLog,
		Pool:           pwm.CandidatePool,
		RealModelName:  in.Before.Model,
		Model:          pwm.Model,
		MaxRetry:       getRequestMaxRetry(ctx),
		UpstreamWindow: window,
	})

	switch outcome.Status {
	case retryLoopSucceeded:
		return outcome.Result, nil
	case retryLoopAborted:
		return nil, outcome.Err
	default:
		return nil, retryExhaustedOutcome(ctx, "maximum retry attempts reached")
	}
}
