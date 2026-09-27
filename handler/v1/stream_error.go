package v1

import (
	"errors"
	"fmt"
	"io"

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
func writeResponseBody(c *gin.Context, style string, stream bool, src io.Reader) error {
	if _, err := io.Copy(c.Writer, src); err != nil {
		if stream {
			if emitErr := writeStreamErrorEvent(c, style, streamInterruptMessage); emitErr != nil {
				// 错误事件构造/写出失败不静默：并入返回错误，交日志管道唯一留痕。
				err = errors.Join(err, emitErr)
			}
		}
		return fmt.Errorf("write response body failed: %w", err)
	}
	return nil
}

// writeStreamErrorEvent 在响应头（200/SSE）已提交后向客户端写出该协议形状的
// SSE error 事件收尾。此时状态码已无法改写，httpresp.InternalServerError 的 JSON
// 会不带 SSE 前缀原样污染已提交的流（客户端的 SSE 解析器会把它当垃圾数据），
// 协议内错误事件才能被识别为失败。message 必须是静态文案（禁拼内部错误文本）。
//
// 形状配方按 style 注册（service.GetStreamErrorWriter，Stage 30 W5 收口：原先这里是
// 手写 switch、新 style 漏补分支会静默退化为只断流；现注册面有完备性机械锁）。
// 未知 style / 未登记配方：无协议可依，仅断流不写字节。帧内 JSON 转义由配方侧
// typed marshal 兜底；构造失败原样上抛（并入 writeResponseBody 的返回错误）。
func writeStreamErrorEvent(c *gin.Context, style string, message string) error {
	writer, err := service.GetStreamErrorWriter(style)
	if err != nil {
		c.Writer.Flush()
		return nil
	}
	frame, err := writer(message)
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
