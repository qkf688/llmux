package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/middleware"
	authservice "github.com/qkf688/llmux/service/auth"
)

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword 校验旧密码后更新为新密码。
func ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "old_password and new_password are required")
		return
	}

	user := middleware.CurrentUser(c)
	if user == nil {
		httpresp.Unauthorized(c, "not authenticated")
		return
	}

	if err := authservice.VerifyPassword(user.PasswordHash, req.OldPassword); err != nil {
		httpresp.Unauthorized(c, "old password incorrect")
		return
	}

	hash, err := authservice.HashPassword(req.NewPassword)
	if err != nil {
		httpresp.InternalServerError(c, "failed to hash password")
		return
	}

	if err := repos().User.UpdatePassword(c.Request.Context(), user.ID, hash); err != nil {
		httpresp.InternalServerError(c, "failed to update password")
		return
	}

	httpresp.Success(c, nil)
}
