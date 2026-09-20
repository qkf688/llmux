package middleware

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/common"
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

// accessLogLineMaxBytes 访问行单行上限（Stage 16）：path 段含解码后原文 + 原始
// query，无上限时病态 URL（仅受 net/http MaxHeaderBytes ~1 MiB 兜底）会把 MiB 级
// 单行写进 stdout。体量防御类上限走代码常量不做设置（consts/limits.go 口径：
// 可被调小的防线不是防线）。
const accessLogLineMaxBytes = 4 << 10

// formatAccessLine 组装访问行。Stage 16 单行上限采用「path 段预算」口径（方案 B）：
// 超限时截断施加在 %q **之后**的 path 段，而非整行尾——病态长 URL 恰恰是最需要
// 排障关联的请求，整行尾截会把 status/耗时/rid 这些关联键切掉；预算 = 整行上限 −
// 其余字段实测长度 − 截断标记长度（TruncateUTF8 的标记追加在预算字节之外）。
// 先 %q 后截：控制字符的 \xNN 展开（最坏 4 倍）会让「先截后转义」的行重新超限。
// 截断实现单源 common.TruncateUTF8（rune 边界回退 + 标记），此处只定预算与组装。
func formatAccessLine(p gin.LogFormatterParams) string {
	quotedPath := strconv.Quote(p.Path) // 与原 %q 同源
	line := assembleAccessLine(p, quotedPath)
	if len(line) <= accessLogLineMaxBytes {
		return line
	}
	budget := accessLogLineMaxBytes - (len(line) - len(quotedPath)) - len(common.TruncatedSuffix)
	if budget >= 1 {
		if capped := assembleAccessLine(p, common.TruncateUTF8(quotedPath, budget)); len(capped) <= accessLogLineMaxBytes {
			return capped
		}
	}
	// 兜底（审查回批 W1）：path 段预算只能约束 path，其余字段（ClientIP/Method）无
	// 长度约束——gin 默认信任全部代理且本项目未 SetTrustedProxies（Stage 20 议题），
	// XFF 伪造的 ClientIP 可达任意长度，path 预算压不下整行。此时退化为整行尾部截断：
	// 体量上界是防线本体，rid/status 只在载体字段本身已病态时被切（放弃关联键换上界，
	// 比把无上限单行放进 stdout 可控）。截断预算同样扣标记长度，且保留尾部换行——
	// TruncateUTF8 的标记追加在预算之外，不扣长度兜底自身又超限、切掉 \n 则下一条
	// 访问行会拼接进来破坏「一行请求 = 一行日志」。
	body := common.TruncateUTF8(strings.TrimSuffix(line, "\n"),
		accessLogLineMaxBytes-len(common.TruncatedSuffix)-1)
	return body + "\n"
}

func assembleAccessLine(p gin.LogFormatterParams, quotedPath string) string {
	return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %s | rid=%s\n",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"),
		p.StatusCode,
		p.Latency.Truncate(time.Microsecond),
		p.ClientIP,
		p.Method,
		quotedPath,
		p.Request.Header.Get(consts.HeaderRequestID), // 已经过 RequestID 白名单校验，无需转义
	)
}
