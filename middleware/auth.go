package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/auth"
)

// ContextKey 当前用户在 gin context 中的键。
const ContextKeyUser = "currentUser"

// AuthJWT 校验 Bearer JWT，通过后注入 *models.User 到 context。
func AuthJWT(secret string, repo repository.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Authorization header is missing")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid authorization header")
			c.Abort()
			return
		}

		claims, err := auth.Parse(secret, parts[1])
		if err != nil {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid or expired token")
			c.Abort()
			return
		}

		user, err := repo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "User not found")
			c.Abort()
			return
		}

		c.Set(ContextKeyUser, user)
		c.Next()
	}
}

// AuthAPIKey 校验 /v1 代理 API 的 per-user API key。
// 优先 Authorization: Bearer <key>，回退 x-api-key（Anthropic 兼容）。
func AuthAPIKey(repo repository.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := extractAPIKey(c)
		if key == "" {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Authorization header or x-api-key header is missing")
			c.Abort()
			return
		}

		user, err := repo.FindByAPIKey(c.Request.Context(), key)
		if err != nil {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid API key")
			c.Abort()
			return
		}

		c.Set(ContextKeyUser, user)
		c.Next()
	}
}

// extractAPIKey 从 Authorization Bearer 或 x-api-key 头提取 API key。
func extractAPIKey(c *gin.Context) string {
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	return c.GetHeader("x-api-key")
}

// CurrentUser 从 gin context 取当前用户（已通过 AuthJWT 或 AuthAPIKey 注入）。
func CurrentUser(c *gin.Context) *models.User {
	v, ok := c.Get(ContextKeyUser)
	if !ok {
		return nil
	}
	user, _ := v.(*models.User)
	return user
}
