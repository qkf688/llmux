package httpresp

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

// InternalServerError 内部服务器错误
func InternalServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Error:   message,
		Message: message,
	})
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
