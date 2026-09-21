package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

// 合法入站编号内核：仅字母数字下划线连字符，1~64 位。超长或含控制字符/空白/
// 非 ASCII 的一律弃用重生成——服务端日志字段不许被外部输入污染（log injection）。
// 校验对象是内核，不含来源前缀：前缀由本包统一拼接（Stage 17），外部自选的
// gw:/in: 前缀值走弃用分支。
var validRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{1,64}$`)

// RequestID 为每个请求确定唯一编号：自生成打 gw: 前缀，沿用调用方自带且内核
// 格式规矩的 X-Request-Id 打 in: 前缀（Stage 17 来源标记——同一字段里可区分
// 「网关生成」与「调用方自带」，外部自选值不再与自生成号共桶串号；沿用内核
// 让调用方自家 trace 体系与服务端日志直接对上）。前缀进值本体而非展示点拼接：
// access_log formatter 只拿得到请求头，落库/响应头/前端补号各自拼必然漂移。
// 编号写三处——gin.Context（错误日志取用）、响应头（用户报障时拿得走）、
// 入站请求头（访问日志与 /v1 上游透传白名单都从请求头取值，三处同一个号）。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// c.Request 由 gin 引擎保证非 nil；此处判空只护裸 Context 单测，
		// 与 httpresp.logInternalError 的 nil 姿态保持一致。
		inbound := ""
		if c.Request != nil {
			inbound = c.Request.Header.Get(consts.HeaderRequestID)
		}
		id := consts.RequestIDPrefixGenerated + generateRequestID()
		switch {
		case strings.HasPrefix(inbound, consts.RequestIDPrefixGenerated),
			strings.HasPrefix(inbound, consts.RequestIDPrefixInbound):
			// 来源前缀是网关标记，外部不得自选：入站已带前缀一律弃用重生成，
			// 防 in:gw:… 双前缀与 gw: 自生成号空间被外部污染。
		case validRequestIDPattern.MatchString(inbound):
			id = consts.RequestIDPrefixInbound + inbound
		}
		c.Set(consts.ContextKeyRequestID, id)
		if c.Request != nil {
			c.Request.Header.Set(consts.HeaderRequestID, id)
		}
		c.Header(consts.HeaderRequestID, id)
		c.Next()
	}
}

// generateRequestID 生成 12 位小写 hex 内核（48 bit 熵，crypto/rand）。单实例
// 排障够用；不引第三方 ID 库（低影响可换库自己定）。来源前缀由 RequestID
// 统一拼接，此处只产内核。
func generateRequestID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败属系统级异常，退化为纳秒时间戳低位——绝不能退化成
		// 常量哨兵：那会让并发请求全部撞号，且常量值可被入站伪造混入同一桶。
		return fmt.Sprintf("%012x", uint64(time.Now().UnixNano())&0xFFFFFFFFFFFF)
	}
	return hex.EncodeToString(b[:])
}
