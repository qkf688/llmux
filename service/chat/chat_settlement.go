package chat

import (
	"context"
	"errors"
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
// 平台错误码猜：上游自己把连接掉了是真实上游故障，不该被当成调用方放弃。超时
// （DeadlineExceeded）与取消语义不同——超时是上游不吭声，仍归上游/端点故障照常结算；
// 两者几乎同时发生时按超时算（宁可错杀）。派生的写库 ctx 与日志同款（WithoutCancel 摘掉
// 取消信号、独立短超时防挂死），沿用 chatLogDetachedCtx 的单源派生表达式。

// failureSettlement 失败结算的裁决：Active=false 表示调用方已放弃本次请求，调用方
// 只保留已写出的失败留痕，不写任何结算字段。
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
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	return failureSettlement{Ctx: writeCtx, Active: true, cancel: cancel}
}

// requestAbandoned 判定请求是否被调用方主动放弃：只认请求 ctx 的取消信号。
func requestAbandoned(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}

// requestAbandonedResult 调用方放弃后的终态：留痕（失败日志行）已写，随即中止本次
// 请求——不再换 key、不再换供应商，调用方已经走了，换了也没人收。
func requestAbandonedResult(ctx context.Context) singleProviderAttemptResult {
	return singleProviderAttemptResult{FatalErr: ctx.Err()}
}

// successWriteCtx 成功侧结算 ctx：上游已经成功是既成事实，成功记账与失败计数清零不因
// 调用方恰在此刻断开而作废（与失败侧相反——失败侧要先判断这次失败是不是调用方造成的）。
func successWriteCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return chatLogDetachedCtx(ctx)
}
