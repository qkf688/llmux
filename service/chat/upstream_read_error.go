package chat

import (
	"errors"
	"io"
)

// 读侧错误标记：让「上游故障」的方向信息穿过日志管道到达判定层。
//
// 背景：`handler/v1` 的 writeResponseBody 用一次 io.Copy 把上游响应体写给客户端，
// 它返回的 error **不携带方向**——读侧失败（上游腰斩/断连）与写侧失败（客户端撤走）
// 是同一个对象。提交后失败的清钉判定必须区分这两者（前者该忘、后者不该忘），而
// 既有判据 isClientDisconnectErr 只能从错误**类型**反推：它把任何 Timeout()==false
// 的 net.Error 一律判为客户端断连，于是「上游 RST」（*net.OpError，wsarecv forcibly
// closed）被误判成客户端撤走 → 该清的没清。真机实测：chunked 截断（io.ErrUnexpectedEOF）
// 判对会清，RST 判错不清。
//
// 正解不是在判定层猜，而是在**错误产生处**把方向记下来：本文件的 MarkUpstreamReader
// 包装上游 reader，读失败时给错误套一层 upstreamReadErr，判定层优先看标记。
//
// 为什么不靠「上游ReadCloser 不实现 net.Error」隐式生效：upstreamReadErr 实现了
// Unwrap，errors.As 仍会沿链找到内层的 net.Error，隐式方案与显式判定无法区分二者，
// 判据会随内层类型漂移。故显式：带标记 ⇒ 上游故障。

// upstreamReadErr 标记「此错误来自读上游」。刻意不实现 net.Error：内层错误类型
// 仍可通过 errors.Is/As 触达（保留诊断能力），但方向由本类型承载而非类型推断。
type upstreamReadErr struct{ err error }

func (e upstreamReadErr) Error() string { return e.err.Error() }
func (e upstreamReadErr) Unwrap() error { return e.err }

// IsUpstreamReadErr 判定错误是否来自读上游（上游侧故障）。优先级高于
// isClientDisconnectErr：标记是错误产生处记下的方向事实，而后者只是类型反推的猜测。
func IsUpstreamReadErr(err error) bool {
	var e upstreamReadErr
	return errors.As(err, &e)
}

// MarkUpstreamReader 给 reader 的**读**错误加方向标记。调用点必须放在 tee **之外**
// （即先包 reader 再 TeeReader）：tee 往日志管道的写失败（如日志侧提前收尾导致的
// io.ErrClosedPipe）不属于上游故障，混进标记会把「日志管道收尾」误判成「上游腰斩」。
func MarkUpstreamReader(r io.Reader) io.Reader { return upstreamReader{r} }

type upstreamReader struct{ io.Reader }

func (r upstreamReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil {
		return n, upstreamReadErr{err}
	}
	return n, nil
}
