package chat

import (
	"context"
	"errors"
	"io"
)

// 写侧错误标记：让「客户端侧失败」的方向信息穿过日志管道到达判定层。
//
// 背景（Stage 39）：`handler/v1` 的 writeResponseBody 用一次 io.Copy 把响应体写给
// 客户端。写回从 Stage 39 起有**写上限**（钉到请求总时限），到点时 io.Copy 返回一个
// 带 Timeout()==true 的写错误——而判定层的 isClientDisconnectErr 明确把超时排除在
// 断连之外（Stage 33：超时归「服务端或上游掐断」），于是「客户端不读结果、被写上限
// 掐断」会被当成供应商故障：清掉它的粘性会话记忆、给它记一次失败统计。供应商答得
// 好好的，是客户端不收，凭什么罚它。
//
// 与读侧同构：方向在**错误产生处**记下来（upstream_read_error.go 的 MarkUpstreamReader
// 是同一条路的读侧版本），判定层优先看标记，而不是从错误类型反推——写超时的错误形状
// 与上游超时的错误形状在类型层无法区分。

// clientWriteErr 标记「此错误来自写客户端」。刻意不实现 net.Error：内层错误类型仍可
// 通过 errors.Is/As 触达（保留诊断能力），但方向由本类型承载而非类型推断。
type clientWriteErr struct{ err error }

func (e clientWriteErr) Error() string { return e.err.Error() }
func (e clientWriteErr) Unwrap() error { return e.err }

// IsClientWriteErr 判定错误是否来自写客户端（调用方侧失败）。优先级高于
// isClientDisconnectErr：标记是错误产生处记下的方向事实。
func IsClientWriteErr(err error) bool {
	var e clientWriteErr
	return errors.As(err, &e)
}

// MarkResponseWriteError 给「写回响应」这一步的错误打方向标记，供 RecordLog 的提交后
// 失败判定区分「供应商故障」与「客户端侧失败」（清粘性钉定 + 供应商失败统计 + 留痕
// 级别三处共用同一判据，见 chat_record.go 的 clientSideFailure）。
//
// ctx 是该请求的 ctx（总时限挂在它的 deadline 上），下面第 2 条判据要读它。
//
// 1. 读侧错误已带 upstreamReadErr 标记，原样放过：io.Copy 返回的错误既可能是读 src 失败
// 也可能是写 dst 失败，两者在错误链里都只是一枚 err，此时**不能**盖上写侧标记——
// 覆盖会把「上游腰斩」误判成「客户端不收」，粘性钉定该清的不清、供应商白背锅。
//  2. 总时限已耗尽（ctx.Err()==DeadlineExceeded）不盖：写上限钉的就是 ctx 的 deadline，
//     预算到点那一刻 socket deadline 与 ctx deadline 同时触发，第一次写立刻失败——那是
//     「预算烧完」，不是「客户端不收」，须保留既有的「超时归供应商」记账（2026-10-04
//     拍板）。**只认 DeadlineExceeded、不认 Canceled**：客户端中途断开是 ctx 被取消，
//     那确实是客户端侧，仍须打标。
//  3. io.ErrClosedPipe 不盖：handler/v1/chat.go 的 io.TeeReader 刻意在方向标记**之外**
//     （管道写失败不是上游腰斩），故日志侧提前收尾时 pw.Write 的失败是**没有方向标记的
//     第三种失败源**，默认会被兜成「客户端」；但它的诱因通常是上游吐了让解析器提前退出
//     的内容，账该记在上游（2026-10-04 拍板）。注意这**只**改统计面：清钉与留痕级别
//     两处走 clientSideFailure，其中 isClientDisconnectErr 自 Stage 24（commit 4d1264a
//     「断连类降 Warn」）起就把 io.ErrClosedPipe 直接算断连，与本标记是两条独立判据，
//     改那一行属动既有口径，待拍板。
func MarkResponseWriteError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if IsUpstreamReadErr(err) {
		return err
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return err
	}
	return clientWriteErr{err}
}
