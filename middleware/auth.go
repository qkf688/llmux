package middleware

import (
	"net/http"
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/gin-gonic/gin"
)

func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 不设置token，则不进行验证
		if token == "" {
			return
		}
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			common.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Authorization header is missing")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			common.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid authorization header")
			c.Abort()
			return
		}

		tokenString := parts[1]
		if tokenString != token {
			common.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid token")
			c.Abort()
			return
		}
	}
}

func AuthAnthropic(koken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 不设置token，则不进行验证
		if koken == "" {
			return
		}

		// 优先检查 Authorization: Bearer 头
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" && parts[1] == koken {
				return
			}
		}

		// 回退检查 x-api-key 头（保持向后兼容）
		xApiKey := c.GetHeader("x-api-key")
		if xApiKey == "" {
			common.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Authorization header or x-api-key header is missing")
			c.Abort()
			return
		}
		if xApiKey != koken {
			common.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid token")
			c.Abort()
			return
		}
	}
}
