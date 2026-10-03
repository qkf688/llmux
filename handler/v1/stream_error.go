package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/service"
)

// streamInterruptMessage 是「提交后断流」写给客户端的静态文案。
// 500 收口纪律（framework-conventions §5）：禁拼错误文本，客户端只能拿到静态文案。
const streamInterruptMessage = "upstream stream interrupted before completion"

// writeResponseBody 把已验证的上游响应体写给客户端。响应头提交后任何失败都不能再走
// httpresp 的 JSON 收口（信封会污染已开始的流/残缺 JSON），收尾只此两种：
//   - 流式：写协议内 error 事件收尾（客户端解析器可识别为失败）；
//   - 非流式：静默截断（残缺 JSON 本身就是失败信号，再拼任何字节都是污染）。
//
// 两者都不在 handler 留痕：返回的首个错误经日志管道（pw.CloseWithError → RecordLog）
// 写 chat_logs Status=error，并留该次失败的**唯一一条**留痕（断连类 Warn、其余 ERROR，
// 级别判定在 RecordLog 收口）——handler 再自记即双份，按行计数的告警会把一次故障
// 算成两次（framework-conventions §5 边界③，2026-09-23 双份留痕回批）。
// 写回错误打上「来自客户端」方向标记（service.MarkResponseWriteError）：读上游失败
// 与写客户端失败在 io.Copy 返回的 error 里是同一枚，分不开方向；不标记的话，提交后
// 失败的清钉与供应商失败统计会把「客户端不收结果」算到供应商头上（Stage 39，R3）。
// 打标函数自带三条例外（已带读侧标记 / 总时限已耗尽 / 日志管道收尾不打标），故本函数
// 不自己判方向——判定单源在 service/chat，别在这里复制一份。
func writeResponseBody(ctx context.Context, c *gin.Context, style string, stream bool, src io.Reader) error {
	applyClientWriteDeadline(c, ctx)

	if _, err := io.Copy(c.Writer, src); err != nil {
		if stream {
			if emitErr := writeStreamErrorEvent(c, style); emitErr != nil {
				// 错误事件构造/写出失败不静默：并入返回错误，交日志管道唯一留痕。
				err = errors.Join(err, emitErr)
			}
		}
		return fmt.Errorf("write response body failed: %w", service.MarkResponseWriteError(ctx, err))
	}
	return nil
}

// applyClientWriteDeadline 把「可写到的时刻」钉成该请求的总时限（ctx 上由
// service.WithRequestTotalTimeout 挂的那个 deadline）。
//
// 缺了它，一个只发请求、之后再也不读结果的客户端会让 writeResponseBody 永久阻塞：
// 程序卡在一次 Write 里出不来，而「到点必掐」在写方向上并不成立——收上游那半早就
// 完成了，到点检查看不到任何异常。实测（Stage 39）：不加本函数，「发请求后不读响应」
// 的客户端能一直占着处理线程与上游连接。
//
// 用 http.NewResponseController 而非 http.Server.WriteTimeout：后者是**全局**值，
// 会连带砍掉正常的长流式响应（流式输出本来就可能跑满总时限）；这里是请求级语义，
// 与 request_total_timeout 同源——总时限调小，写上限跟着调小。gin 的 responseWriter
// 实现了 Unwrap，ResponseController 能穿透到真实的 ResponseWriter。
//
// 刻意**不在写完时清零**（曾写过一段手动清理，是多余的）：net/http 在每个请求收尾
// 自己清——server.go 的 `conn.serve` 请求循环里 `w.finishRequest()` 紧跟
// `c.rwc.SetWriteDeadline(time.Time{})`。故同一 keep-alive 连接的下一次请求不会带着
// 已过期的写上限（过期不可再延长）。若将来改走 HTTP/2 或换掉 net/http 的 server
// 实现，须重新核对那一行是否仍在。
func applyClientWriteDeadline(c *gin.Context, ctx context.Context) {
	deadline, ok := ctx.Deadline()
	if !ok {
		// 无总时限即无写上限。入口（chatHandler）必挂 deadline，此处只是不 panic 的兜底。
		return
	}
	// ErrNotSupported = ResponseController 穿不到真实的 net.Conn，生产上有两条触发面：
	// 测试里的 recorder，以及**被中间件包了一层且没转发 Unwrap 的 ResponseWriter**——
	// gin-contrib/gzip 的 gzipWriter 就是（实测探针确认返回 ErrNotSupported）。本项目
	// /v1/ 走 gzip.WithExcludedPaths 排除，故聊天路径不受影响；给别的管理端路径复用本
	// 函数前须先确认它没被 gzip 包住，否则上限会静默失效。其余失败是运行期真降级
	// （客户端不收时可能无限阻塞），留一条痕。
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(deadline); err != nil &&
		!errors.Is(err, http.ErrNotSupported) {
		slog.Warn("set client write deadline failed, response may block on a slow client", "error", err)
	}
}

// writeStreamErrorEvent 在响应头（200/SSE）已提交后向客户端写出该协议形状的
// SSE error 事件收尾。此时状态码已无法改写，httpresp.InternalServerError 的 JSON
// 会不带 SSE 前缀原样污染已提交的流（客户端的 SSE 解析器会把它当垃圾数据），
// 协议内错误事件才能被识别为失败。
//
// 文案恒为 streamInterruptMessage 常量，且函数**不收 message 参数**——结构性保证
// 「客户端错误文本不携带内部错误信息」（拼接错误文本直出客户端无从发生，同 Stage 22
// 渲染侧取静态常量的口径）；将来需要第二种文案时加审查过的静态常量，不开放任意串。
//
// 形状配方按 style 注册（service.GetStreamErrorWriter，Stage 30 W5 收口：原先这里是
// 手写 switch、新 style 漏补分支会静默退化为只断流；现注册面有完备性机械锁）。
// 未知 style / 未登记配方：无协议可依，仅断流不写字节。帧内 JSON 转义由配方侧
// typed marshal 兜底；构造失败原样上抛（并入 writeResponseBody 的返回错误）。
func writeStreamErrorEvent(c *gin.Context, style string) error {
	writer, err := service.GetStreamErrorWriter(style)
	if err != nil {
		// 未知 style / 未登记配方：无协议可依，仅断流不写字节。留一条 Warn 作运行期
		// 唯一信号（客户端收不到错误事件本身无声无息；注册完备锁只覆盖已登记 style）。
		slog.Warn("no stream error writer registered, breaking stream without error event", "style", style)
		c.Writer.Flush()
		return nil
	}
	frame, err := writer(streamInterruptMessage)
	if err != nil {
		c.Writer.Flush()
		return fmt.Errorf("build stream error event: %w", err)
	}
	if _, err := c.Writer.Write(frame); err != nil {
		return fmt.Errorf("write stream error event: %w", err)
	}
	c.Writer.Flush()
	return nil
}
