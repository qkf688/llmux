package v1

import (
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
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
func writeResponseBody(c *gin.Context, style string, stream bool, src io.Reader) error {
	if _, err := io.Copy(c.Writer, src); err != nil {
		if stream {
			writeStreamErrorEvent(c, style, streamInterruptMessage)
		}
		return fmt.Errorf("write response body failed: %w", err)
	}
	return nil
}

// writeStreamErrorEvent 在响应头（200/SSE）已提交后向客户端写出该协议形状的
// SSE error 事件收尾。此时状态码已无法改写，httpresp.InternalServerError 的 JSON
// 会不带 SSE 前缀原样污染已提交的流（客户端的 SSE 解析器会把它当垃圾数据），
// 协议内错误事件才能被识别为失败。message 必须是静态文案（禁拼内部错误文本）。
// 三种入站格式的形状各不相同：Anthropic / Responses 用 event: error 行，
// OpenAI Chat 的流式错误是 data: 行 + error 对象。两种形状都不再跟随终止事件
// （错误事件即流尾；各家 SDK 对「无终止事件的流」如何收尾未经真机验证，
// 见 framework-conventions §6 已知边界）。
//
// ⚠️ 新协议 style 必须在此补形状（AGENTS §6「新协议 style」清单第 5 处）：
// 漏改会静默落进 default 分支——客户端拿不到任何错误信号且无机械门会红。
func writeStreamErrorEvent(c *gin.Context, style string, message string) {
	switch consts.Style(style) {
	case consts.StyleAnthropic:
		// 断流不是过载：overloaded_error 会被 Anthropic SDK 判为可重试并整请求重放。
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			`{"type":"error","error":{"type":"api_error","message":"`+message+`"}}`)
	case consts.StyleOpenAI:
		fmt.Fprintf(c.Writer, "data: %s\n\n",
			`{"error":{"message":"`+message+`","type":"server_error","param":null,"code":500}}`)
	case consts.StyleOpenAIRes:
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			`{"type":"error","error":{"code":"server_error","message":"`+message+`"}}`)
	default:
		// 未知 style：无协议可依，仅断流。
	}
	c.Writer.Flush()
}
