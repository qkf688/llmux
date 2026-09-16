package httpresp

import (
	"log/slog"
	"net/http"

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
// 新代码禁止绕过本函数用 c.JSON(500, gin.H{...}) 手搓错误响应（AGENTS 5.x：响应走 httpresp）。
func InternalServerError(c *gin.Context, err error) {
	rid := c.GetString(consts.ContextKeyRequestID)
	logInternalError(c, rid, err)
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
// docs/guides/framework-conventions.md 500 收口节）；err 为仅服务端可见的原始错误，可为 nil。
func InternalErrorMessage(c *gin.Context, message string, err error) {
	rid := c.GetString(consts.ContextKeyRequestID)
	logInternalError(c, rid, err)
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Error:   message,
		Message: message,
	})
}

// logInternalError 统一 500 的服务端留痕：结构化记 request_id、method、path 与原始 err。
// err 为 nil 时不写 error 字段（避免日志出现 error="<nil>" 噪声）。
func logInternalError(c *gin.Context, rid string, err error) {
	attrs := []any{"request_id", rid}
	if c.Request != nil {
		attrs = append(attrs, "method", c.Request.Method)
		if c.Request.URL != nil {
			attrs = append(attrs, "path", c.Request.URL.Path)
		}
	}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	slog.Error("internal server error", attrs...)
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
