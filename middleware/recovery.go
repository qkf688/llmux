package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
)

// Recovery 替代 gin.Recovery（Stage 3）：panic 恰恰是「本机自己坏」的 500，
// gin 默认行为却回**空 body**（调用方拿不到编号）且栈只写 stdout（不带 rid、
// 不过 slog 门面）——正是本 Stage 要消灭的两半毛病。这里统一为：
// 栈经 slog.Error 留痕（带 request_id），响应头未写出时走 httpresp 收口出口
// （客户端拿到 [req=] 文案），已提交（SSE 半开流）则不追加任何 body。
//
// http.ErrAbortHandler 原样再抛给 net/http（静默断连、不打栈）——与 gin.Recovery
// 行为一致：io 泵用它主动掐流是控制流不是故障。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			if err, ok := r.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(r)
			}
			rid := c.GetString(consts.ContextKeyRequestID)
			attrs := []any{"request_id", rid}
			if c.Request != nil {
				attrs = append(attrs, "method", c.Request.Method)
				if c.Request.URL != nil {
					attrs = append(attrs, "path", c.Request.URL.Path)
				}
			}
			slog.Error("panic recovered", append(attrs, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))...)
			if !c.Writer.Written() {
				httpresp.InternalServerError(c, fmt.Errorf("panic: %v", r))
			}
		}()
		c.Next()
	}
}
