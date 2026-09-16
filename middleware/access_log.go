package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

// AccessLog 替代 gin 默认访问日志（gin.Default 里的 Logger），差异有二：每行带上
// 请求编号（排障时访问行与 InternalServerError 的错误行按 rid 对得上），以及**转义**
// 路径——gin 默认 formatter 用 %#v 打印 path 天然免疫换行注入，自定义后必须自己
// %q，否则百分号编码的 %0A 解码成真换行后可伪造整行访问记录（一次请求两行）。
// 必须挂在 RequestID 之后（RequestID 回写请求头，这里读的是入站头快照）。
// 刻意不输出 gin 的 ErrorMessage 通道（全仓无 c.Error() 使用者，500 留痕唯一入口是 httpresp）。
func AccessLog() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: formatAccessLine,
	})
}

func formatAccessLine(p gin.LogFormatterParams) string {
	return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %q | rid=%s\n",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"),
		p.StatusCode,
		p.Latency.Truncate(time.Microsecond),
		p.ClientIP,
		p.Method,
		p.Path,
		p.Request.Header.Get(consts.HeaderRequestID), // 已经过 RequestID 白名单校验，无需转义
	)
}
