package chat

import (
	"context"
	"errors"
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
}

func BalanceChat(ctx context.Context, in BalanceInput) (*BalanceResult, error) {
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

	timer := time.NewTimer(time.Second * time.Duration(pwm.TimeOut))
	defer timer.Stop()

	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:           ctx,
		Start:         in.Start,
		Style:         in.Style,
		Before:        in.Before,
		ReqMeta:       in.ReqMeta,
		IOLog:         pwm.IOLog,
		Pool:          pwm.CandidatePool,
		RealModelName: in.Before.Model,
		Model:         pwm.Model,
		MaxRetry:      pwm.MaxRetry,
		ClientTimeout: time.Second * time.Duration(pwm.TimeOut),
		Deadline:      timer.C,
		DeadlineErr:   errors.New("retry time out"),
	})

	switch outcome.Status {
	case retryLoopSucceeded:
		return outcome.Result, nil
	case retryLoopAborted:
		return nil, outcome.Err
	default:
		return nil, errors.New("maximum retry attempts reached")
	}
}
