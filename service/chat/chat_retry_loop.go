package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/chatcore"
)

// retryLoopInput 描述在**单个候选池**上执行重试循环所需的全部输入。
// 真实模型路径只有一个候选池（来自 ProvidersWithMeta）；虚拟模型路径为每个
// 真实模型各构建一个候选池，逐个调用本循环。两条路径的差异全部收敛为本结构体
// 的字段，循环体内不含路径判断。
type retryLoopInput struct {
	Ctx     context.Context
	Start   time.Time
	Style   string
	Before  Before
	ReqMeta models.ReqMeta
	IOLog   bool

	// Pool 候选池。Pool.WeightItems / Pool.PriorityItems 会被循环**就地修改**
	// （淘汰失败候选），调用方传入后不应再复用其内容做其它判断。
	Pool CandidatePool

	// RealModelName / Model 标识本候选池所属的真实模型：真实路径为请求模型本身，
	// 虚拟路径为当前正在尝试的 ordered model。
	RealModelName string
	Model         *models.Model

	MaxRetry      int
	ClientTimeout time.Duration

	// Deadline 超时信号：真实路径为本次请求的局部计时器，虚拟路径为跨真实模型
	// 共享的全局计时器，故由调用方持有并传入通道而非在此创建。
	Deadline <-chan time.Time
	// DeadlineErr 为 Deadline 触发时返回的错误；两条路径语义不同（单模型重试超时
	// vs 虚拟模型全局超时），由调用方给定。
	DeadlineErr error

	// LogAttrs 追加到本循环所有选路日志的上下文键值（虚拟路径带
	// virtual_model / real_model，便于在多真实模型间区分）。
	LogAttrs []any
}

type retryLoopStatus int

const (
	// retryLoopSucceeded 某个候选转发成功，Result 有效。
	retryLoopSucceeded retryLoopStatus = iota
	// retryLoopExhausted 候选池穷尽：已无可选候选，或 MaxRetry 用尽。
	// 语义由调用方决定——真实路径视为整体失败，虚拟路径换下一个真实模型。
	retryLoopExhausted
	// retryLoopAborted ctx 取消、超时，或遇到不可重试错误，须直接终止整个请求。
	retryLoopAborted
)

type retryLoopOutcome struct {
	Status retryLoopStatus
	// Result 仅在 Status == retryLoopSucceeded 时有效。
	Result *BalanceResult
	// Err 仅在 Status == retryLoopAborted 时有效。
	Err error
}

// runProviderRetryLoop 在给定候选池上执行「选路 → 尝试 → 淘汰」重试循环。
//
// retryLog 通道的生命周期完全由本函数持有：一个候选池对应一条通道，循环退出即
// 关闭。调用方无需（也不应）手动 close——虚拟路径此前在 5 个返回分支各写一次
// close，漏一处即泄漏 RecordRetryLog goroutine。
func runProviderRetryLoop(in retryLoopInput) retryLoopOutcome {
	retryLog := make(chan models.ChatLog, in.MaxRetry)
	defer close(retryLog)
	go RecordRetryLog(context.Background(), retryLog, in.Pool.ModelWithProviderMap)

	for retry := range in.MaxRetry {
		select {
		case <-in.Ctx.Done():
			return retryLoopOutcome{Status: retryLoopAborted, Err: in.Ctx.Err()}
		case <-in.Deadline:
			return retryLoopOutcome{Status: retryLoopAborted, Err: in.DeadlineErr}
		default:
		}

		id, err := chatcore.SelectByPriorityAndWeight(in.Pool.WeightItems, in.Pool.PriorityItems)
		if err != nil {
			// 选不出候选说明池已空，属正常穷尽而非系统错误：调用方据此决定
			// 是整体失败还是换下一个真实模型。
			slog.Warn("no selectable provider left", retryLogAttrs(in.LogAttrs, "error", err)...)
			return retryLoopOutcome{Status: retryLoopExhausted}
		}

		modelWithProvider, ok := in.Pool.ModelWithProviderMap[*id]
		if !ok {
			delete(in.Pool.WeightItems, *id)
			continue
		}

		provider := in.Pool.ProviderMap[modelWithProvider.ProviderID]
		chatModel, err := providers.New(provider.Type, provider.Config, provider.Proxy)
		if err != nil {
			// 单个供应商实例化失败（配置损坏等）只淘汰该候选，池中其余候选仍应尝试。
			slog.Error("failed to create provider", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
			delete(in.Pool.WeightItems, *id)
			continue
		}

		client := providers.GetClientWithProxy(in.ClientTimeout, chatModel.GetProxy())
		slog.Info("using provider", retryLogAttrs(in.LogAttrs,
			"provider", provider.Name,
			"model", modelWithProvider.ProviderModel,
			"proxy", chatModel.GetProxy(),
			"retry", retry+1,
		)...)

		result := executeSingleProviderAttempt(singleProviderAttemptInput{
			Ctx:               in.Ctx,
			Start:             in.Start,
			Style:             in.Style,
			Before:            in.Before,
			RealModelName:     in.RealModelName,
			ReqMeta:           in.ReqMeta,
			IOLog:             in.IOLog,
			Retry:             retry,
			Provider:          provider,
			ModelWithProvider: modelWithProvider,
			Model:             in.Model,
			ChatModel:         chatModel,
			Client:            client,
		}, retryLog)

		if result.FatalErr != nil {
			return retryLoopOutcome{Status: retryLoopAborted, Err: result.FatalErr}
		}
		if result.Success {
			return retryLoopOutcome{Status: retryLoopSucceeded, Result: &BalanceResult{
				Response:     result.Response,
				LogID:        result.LogID,
				ProviderName: provider.Name,
				SideChannel:  result.SideChannel,
			}}
		}

		applyProviderSelectionResult(in.Pool.WeightItems, in.Pool.PriorityItems, *id, result)
	}

	return retryLoopOutcome{Status: retryLoopExhausted}
}

// retryLogAttrs 复制基础属性后再追加，避免直接 append 复用底层数组时
// 多次调用互相覆盖已写入的键值。
func retryLogAttrs(base []any, extra ...any) []any {
	attrs := make([]any, 0, len(base)+len(extra))
	attrs = append(attrs, base...)
	return append(attrs, extra...)
}
