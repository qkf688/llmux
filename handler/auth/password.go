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
		// 旧密码填错属「改一下就能过」的请求问题，不是凭据失效：回 400 而非 401。
		// 管理端的全局规则是「401 = 登录过期，清会话回登录页」——若这里回 401，
		// 用户只是打错旧密码就会被踢下线，且刚写的错误提示随跳转一起消失。
		httpresp.BadRequest(c, "old password incorrect")
		return
	}

	hash, err := authservice.HashPassword(req.NewPassword)
	if err != nil {
		httpresp.InternalErrorMessage(c, "failed to hash password", err)
		return
	}

	if err := repos().User.UpdatePassword(c.Request.Context(), user.ID, hash); err != nil {
		httpresp.InternalErrorMessage(c, "failed to update password", err)
		return
	}

	httpresp.Success(c, nil)
}
