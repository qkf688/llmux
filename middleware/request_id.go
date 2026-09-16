package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

// 合法入站编号：仅字母数字下划线连字符，1~64 位。超长或含控制字符/空白/非 ASCII
// 的一律弃用重生成——服务端日志字段不许被外部输入污染（log injection）。
var validRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{1,64}$`)

// RequestID 为每个请求确定唯一编号：入站自带且格式规矩则沿用（让调用方自家的
// trace 体系与服务端日志直接对上），否则生成随机短号。编号写三处——
// gin.Context（错误日志取用）、响应头（用户报障时拿得走）、入站请求头
// （访问日志与 /v1 上游透传白名单都从请求头取值，三处指向同一个号）。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(consts.HeaderRequestID)
		if !validRequestIDPattern.MatchString(id) {
			id = generateRequestID()
		}
		c.Set(consts.ContextKeyRequestID, id)
		c.Request.Header.Set(consts.HeaderRequestID, id)
		c.Header(consts.HeaderRequestID, id)
		c.Next()
	}
}

// generateRequestID 生成 12 位小写 hex（6 字节熵）。单实例排障够用；
// 不引第三方 ID 库（低影响可换库自己定）。
func generateRequestID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败属系统级异常，退化为固定前缀也比 panic 好——
		// 编号缺失只影响日志关联，不影响正确性。
		return "000000000000"
	}
	return hex.EncodeToString(b[:])
}
