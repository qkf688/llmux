package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/middleware"
	authservice "github.com/qkf688/llmux/service/auth"
)

type rotateAPIKeyResponse struct {
	APIKey string `json:"api_key"`
}

// RotateAPIKey 轮换当前用户的 /v1 API key，返回新 key（明文，仅此一次）。
func RotateAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		httpresp.Unauthorized(c, "not authenticated")
		return
	}

	newKey, err := authservice.GenerateAPIKey()
	if err != nil {
		httpresp.InternalServerError(c, "failed to generate api key")
		return
	}

	if err := repos().User.UpdateAPIKey(c.Request.Context(), user.ID, newKey); err != nil {
		httpresp.InternalServerError(c, "failed to update api key")
		return
	}

	httpresp.Success(c, rotateAPIKeyResponse{APIKey: newKey})
}
