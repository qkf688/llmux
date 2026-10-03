package chat

import "errors"

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
// 读侧错误已带 upstreamReadErr 标记，原样放过：io.Copy 返回的错误既可能是读 src 失败
// 也可能是写 dst 失败，两者在错误链里都只是一枚 err，此时**不能**盖上写侧标记——
// 覆盖会把「上游腰斩」误判成「客户端不收」，粘性钉定该清的不清、供应商白背锅。
func MarkResponseWriteError(err error) error {
	if err == nil {
		return nil
	}
	if IsUpstreamReadErr(err) {
		return err
	}
	return clientWriteErr{err}
}
