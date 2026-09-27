package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
)

// Recovery 替代 gin.Recovery（Stage 3）：panic 恰恰是「本机自己坏」的 500，
// gin 默认行为却回**空 body**（调用方拿不到编号）且栈只写 stdout（不带 rid、
// 不过 slog 门面）——正是本 Stage 要消灭的两半毛病。这里统一为：
// 响应头未写出时走 httpresp 收口出口（客户端拿到 [req=] 文案），栈并进该出口的
// 同一行 attrs——一次 panic 只产一条 ERROR（Stage 18 双份留痕收敛，此前这里
// 自留一行再由收口留一行）；已提交（SSE 半开流）不追加任何 body，响应已回写
// 无从补收口，由本函数自留该 panic 的唯一一条 ERROR（带 request_id + 栈）。
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
			// 未提交：走收口出口，栈并进同一行 attrs——一次 panic 只产一条 ERROR
			// （Stage 18 双份留痕收敛：此前这里自留一行再由收口留一行，按 ERROR 行
			// 计数的告警会把一次 panic 算成两次）。
			if !c.Writer.Written() {
				httpresp.InternalServerError(c, fmt.Errorf("panic: %v", r), "stack", string(debug.Stack()))
				return
			}
			// 已提交（SSE 半开流）：响应已回写无从补收口，本行即该 panic 的唯一一条 ERROR。
			rid := c.GetString(consts.ContextKeyRequestID)
			attrs := []any{"request_id", rid}
			if c.Request != nil {
				attrs = append(attrs, "method", c.Request.Method)
				if c.Request.URL != nil {
					// Stage 26：path 过单源上限（common.SlogPathMaxBytes），与
					// httpresp 留痕行同口径——panic 行同样不吃病态长 URL。
					attrs = append(attrs, "path", common.TruncateUTF8(c.Request.URL.Path, common.SlogPathMaxBytes))
				}
			}
			slog.Error("panic recovered", append(attrs, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))...)
		}()
		c.Next()
	}
}
