package chat

import (
	"context"
	"errors"
	"time"

	"github.com/qkf688/llmux/service/adjustment"
)

// 失败/成功「结算」的上下文派生（Stage 24）。
//
// 请求路径的收尾动作分两类：**留痕**（chat_logs 行）与**结算**（调度状态写库——权重/
// 优先级衰减、连续失败、凭据冷却、凭据失败计数与判停）。留痕已在 Stage 17 收口
// （chatLogDetachedCtx），结算此前仍直接挂在请求 ctx 上，两个方向同时错：
//
//   - 调用方主动放弃（客户端断连 → 请求 ctx 收到取消信号）时，本次失败不是上游/凭据的
//     故障，却照旧冷却了无辜 key、给本该正常的端点记了一次失败（用户关页/停止按钮是
//     再平常不过的事件）；
//   - 本服务自己等到超时（deadline 到）时请求 ctx 同样已死，结算写库静默失败——坏供应商
//     不被降权、坏 key 不被避让，恰在最需要保护的故障期失效。
//
// 判据只认**请求 ctx 是否收到取消信号**（客户端断连的唯一可靠信号），不按错误文本或
// 平台错误码猜：上游自己把连接掉了是真实上游故障，不该被当成调用方放弃。超时与取消
// 语义不同——超时是上游不吭声，仍归上游/端点故障照常结算；两者都出现时按超时算
// （宁可错杀）。本链路请求 ctx 的实际状态：客户端/反代撤走 = Canceled（关连接）；
// 等头超时由 http.Client 报超时错误而**请求 ctx 仍活**（照常结算）；网关总超时走重试
// 循环的 Deadline 通道（既有语义：不结算）。请求 ctx 的 DeadlineExceeded 目前不由本项目
// 产生（http.Server 未设超时、网关超时是定时器），判据仍按「超时不算放弃」写，将来有人
// 给请求 ctx 设 deadline 时行为正确。

// settlementWriteTimeout 结算写库的独立预算：结算是**多语句顺序写**（连续失败计数 →
// 权重衰减 → 优先级衰减 → 凭据侧写；每步都带若干次设置项 SELECT，设置读取无缓存），
// 与「单条日志写」的量级不同，故不与 chatLogWriteTimeout 共用——预算过紧会在语句序列
// 中途到期，留下半截结算（如计数已加、衰减未写）且只有 slog 留痕。取值 ≥ 2×
// SQLite busy_timeout（5s），保证「等锁」与「预算」不会按构造同时到期。
const settlementWriteTimeout = 15 * time.Second

// failureSettlement 失败结算的裁决：Active=false 表示调用方已放弃本次请求，调用方
// 只保留已写出的失败留痕，不写任何结算字段。
//
// 非活跃态刻意让 Ctx 为 nil——忘记看 Active 而直接使用会在 GORM 里当场炸，比「静默用
// Background 写错账」更早暴露（此前的缺陷正是静默写）。
type failureSettlement struct {
	Ctx    context.Context
	Active bool

	cancel context.CancelFunc
}

// Close 释放派生 ctx（Active=false 时无 ctx 可释放）。
func (s failureSettlement) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// beginFailureSettlement 失败结算入口：一次给出「这次失败该不该结算」与「结算写库用的
// ctx」。判断与派生都收在此处，失败出口只调用它——新增出口自动获得同一口径（OCP），
// 不必各自记得判断两次。
func beginFailureSettlement(ctx context.Context) failureSettlement {
	if requestAbandoned(ctx) {
		return failureSettlement{}
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settlementWriteTimeout)
	return failureSettlement{Ctx: writeCtx, Active: true, cancel: cancel}
}

// requestAbandoned 判定请求是否被调用方主动放弃：只认请求 ctx 的取消信号；若取消与
// deadline 同时存在且 deadline 已过，按超时算（不是调用方造成的失败）。
func requestAbandoned(ctx context.Context) bool {
	if !errors.Is(ctx.Err(), context.Canceled) {
		return false
	}
	if dl, ok := ctx.Deadline(); ok && !time.Now().Before(dl) {
		return false
	}
	return true
}

// requestAbandonedResult 调用方放弃后的终态：留痕（失败日志行）已写，随即中止本次
// 请求——不再换 key、不再换供应商，调用方已经走了，换了也没人收。
func requestAbandonedResult(ctx context.Context) singleProviderAttemptResult {
	return singleProviderAttemptResult{FatalErr: ctx.Err()}
}

// successWriteCtx 成功侧结算 ctx：上游已经成功是既成事实，成功记账与失败计数清零不因
// 调用方恰在此刻断开而作废（与失败侧相反——失败侧要先判断这次失败是不是调用方造成的）。
func successWriteCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), settlementWriteTimeout)
}

// settleRequestBuildFailureDecay 上游请求构建期失败的结算（转换请求体失败 / BuildReq 失败）：
// 只做权重与优先级衰减——与既有语义一致，不累计连续失败（失败发生在上游被触达之前，
// 谈不上「连续调用失败」），经 beginFailureSettlement 判定：调用方撤走时同样不罚供应商。
func settleRequestBuildFailureDecay(input singleProviderAttemptInput) {
	settlement := beginFailureSettlement(input.Ctx)
	defer settlement.Close()
	if !settlement.Active {
		return
	}
	adjustment.ApplyWeightDecayByModelProviderID(settlement.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
	adjustment.ApplyPriorityDecayByModelProviderID(settlement.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
}
