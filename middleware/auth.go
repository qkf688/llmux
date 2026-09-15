package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/auth"
	"gorm.io/gorm"
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

// AuthAPIKey 校验 /v1 代理 API 的调用 key（多 api-key 方案 #4-2）。
// 优先 Authorization: Bearer <key>，回退 x-api-key（Anthropic 兼容）；
// 按 SHA-256(明文) 直接 SQL 点查 api_keys（单进程 SQLite 索引点查，不做内存
// 索引——方案 §6 定案）。hash 用 credentialcrypto.HashKey（纯原语，鉴权不依赖
// 加密装配态）。放行后注入 owner 用户（保留 per-user 语义）与 CallerKey 快照。
func AuthAPIKey(apiKeyRepo repository.ApiKeyRepo, userRepo repository.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := extractAPIKey(c)
		if key == "" {
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Authorization header or x-api-key header is missing")
			c.Abort()
			return
		}

		ctx := c.Request.Context()
		callerKey, err := apiKeyRepo.FindByHash(ctx, credentialcrypto.HashKey(key))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "Invalid API key")
			} else {
				// 基础设施故障不伪装成鉴权失败（否则 DB 坏了看起来像所有人 key 都错）；
				// err 只进服务端日志不进响应（不外泄驱动细节，运维需能区分两故障点）
				slog.Error("v1 auth: api key lookup failed", "error", err)
				httpresp.InternalServerError(c, "failed to verify api key")
			}
			c.Abort()
			return
		}
		if !callerKey.Enabled {
			// 独立文案无泄露面：能命中该行本身以持有合法明文为前提
			httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "API key is disabled")
			c.Abort()
			return
		}

		user, err := userRepo.GetByID(ctx, callerKey.OwnerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "api key owner not found")
			} else {
				slog.Error("v1 auth: owner lookup failed", "error", err, "api_key_id", callerKey.ID)
				httpresp.InternalServerError(c, "failed to resolve api key owner")
			}
			c.Abort()
			return
		}

		c.Set(ContextKeyUser, user)
		c.Set(ContextKeyCallerKey, &CallerKey{ID: callerKey.ID, Name: callerKey.Name})
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
