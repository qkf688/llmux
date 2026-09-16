package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

// AccessLog 替代 gin 默认访问日志（gin.Default 里的 Logger），唯一差异是每行带上
// 请求编号：排障时访问行与 InternalServerError 的错误行按 rid 互相对得上。
// 必须挂在 RequestID 之后（RequestID 回写请求头，这里读的是入站头快照）。
func AccessLog() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: nil,
		Formatter: func(p gin.LogFormatterParams) string {
			return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %s | rid=%s\n",
				p.TimeStamp.Format("2006/01/02 - 15:04:05"),
				p.StatusCode,
				p.Latency.Truncate(time.Microsecond),
				p.ClientIP,
				p.Method,
				p.Path,
				p.Request.Header.Get(consts.HeaderRequestID),
			)
		},
	})
}
