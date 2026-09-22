package httpresp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

// Response 统一响应结构
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})
}

// SuccessRaw 成功响应（直接返回 data，不包 Response）
func SuccessRaw(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// SuccessNoStore 成功响应 + 禁止任何缓存层留存（受控明文端点专用）。
// 明文一旦落进浏览器磁盘缓存或中间代理，前端「翻页即作废 / 隐藏即删 / 关弹窗清空」的
// 生命周期守卫就形同虚设——缓存里的响应体比内存 state 活得久得多，且不受前端管辖。
// 与 Success 合并成一个函数，是为了让「返回明文」和「禁缓存」不可能只做一半。
func SuccessNoStore(c *gin.Context, data any) {
	c.Header("Cache-Control", "no-store")
	Success(c, data)
}

// SuccessWithMessage 带消息的成功响应
func SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithHttpStatus 带HTTP状态码的错误响应
func ErrorWithHttpStatus(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// InternalServerError HTTP 500 出口收口（Stage 3）：原始 err 只进服务端 slog
// （带 request_id/method/path），对外一律发内嵌编号的通用文案。响应体因此不含
// 任何内部错误串（表名/列名/约束名），调用方仅凭编号即可让运维在日志中定位本次请求。
// 编号源自 RequestID 中间件；缺席时（如单测直连）退化为不带后缀的恒定文案，形状稳定。
//
// attrs 供调用方把业务上下文（credential_id / dbPath / stack 等）并进**同一行**留痕
// （Stage 18）：一次 500 故障只产这一条 ERROR，业务细节与编号、原始错误同处一行。
// 早先「调用点先手写一行细节、收口再留一行错误」的双份留痕已收敛——两行会让按
// ERROR 行计数的告警翻倍，且细节行还得靠 rid 才能与错误行对上。键值必须成对；
// 保留键 request_id / method / path / message / error 禁止占用。
//
// 新代码禁止绕过本函数用 c.JSON(500, gin.H{...}) 手搓错误响应（AGENTS 5.x：响应走 httpresp），
// 也禁止在调用点另写 slog.Error/Warn 给同一个错误重复留痕（ERROR 唯一入口，
// 见 framework-conventions §5）。
func InternalServerError(c *gin.Context, err error, attrs ...any) {
	rid := c.GetString(consts.ContextKeyRequestID)
	logInternalError(c, rid, "", err, attrs...)
	msg := "internal server error"
	if rid != "" {
		msg = "internal server error [req=" + rid + "]"
	}
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Error:   msg,
		Message: msg,
	})
}

// InternalErrorMessage 已审查静态文案的 500 出口（不含内部信息的固定提示，如
// "encryption not configured"）：对外原样返回该文案（不拼编号），对内仍留痕。
//
// message 参数必须是常量字面量、禁止拼接任何错误文本（一键自查见
// docs/guides/framework-conventions.md 500 收口节）；err 为仅服务端可见的原始错误，可为 nil；
// attrs 与保留键约束同 InternalServerError。
func InternalErrorMessage(c *gin.Context, message string, err error, attrs ...any) {
	rid := c.GetString(consts.ContextKeyRequestID)
	logInternalError(c, rid, message, err, attrs...)
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Error:   message,
		Message: message,
	})
}

// logInternalError 统一 500 的服务端留痕：结构化记 request_id、method、path、
// （静态出口的）message、调用方业务上下文 attrs 与原始 err——**每调用写一行**，
// 这就是一次 500 故障的唯一一条 ERROR（Stage 18：此前调用点常另写一行，同一次
// 故障因此占两条 ERROR，按行计数的告警翻倍）。err 为 nil 时不写 error 字段（避免
// error="<nil>" 噪声）。message 为空（通用出口）则不写 message 字段。
// 客户端断连/上传中断不是服务器故障，降 Warn——否则「建连即断」可把 ERROR
// 通道刷满、按 ERROR 配的告警失灵（log flooding）。
func logInternalError(c *gin.Context, rid string, message string, err error, attrs ...any) {
	fields := []any{"request_id", rid}
	if c.Request != nil {
		fields = append(fields, "method", c.Request.Method)
		if c.Request.URL != nil {
			fields = append(fields, "path", c.Request.URL.Path)
		}
	}
	if message != "" {
		fields = append(fields, "message", message)
	}
	fields = append(fields, attrs...)
	if err != nil {
		fields = append(fields, "error", err)
	}
	if isClientDisconnect(err) {
		slog.Warn("internal server error", fields...)
		return
	}
	slog.Error("internal server error", fields...)
}

// isClientDisconnect 识别「对端先挂断」类错误（ctx 取消 / 坏管道 / 连接重置）：
// 它们是调用方行为而非本服务故障，不该占用 ERROR 通道。
func isClientDisconnect(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET)
}

// BadRequest 请求参数错误
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusBadRequest,
		Message: message,
	})
}

// NotFound 资源未找到
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    404,
		Message: message,
	})
}

// Unauthorized 未授权
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    401,
		Message: message,
	})
}

// Conflict 资源冲突（真实 HTTP 409）。
// 与 BadRequest/NotFound 不同类：那些只把码塞进信封（HTTP 恒 200），而 key 查重等
// 「与既有资源撞唯一约束」的语义需要真实状态码供调用方分支（多 api-key 方案 §5）。
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, Response{
		Code:    http.StatusConflict,
		Message: message,
	})
}

// Forbidden 禁止访问
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    403,
		Message: message,
	})
}
