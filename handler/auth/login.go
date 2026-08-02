package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	authservice "github.com/qkf688/llmux/service/auth"
)

type loginRequest struct {
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type loginResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

// Login 返回登录 handler，secret 通过闭包持有（避免包级可变状态）。
func Login(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			httpresp.BadRequest(c, "password is required")
			return
		}

		repo := repos().User
		user, err := repo.FindByUsername(c.Request.Context(), authservice.AdminUsername)
		if err != nil {
			httpresp.Unauthorized(c, "invalid password")
			return
		}

		if err := authservice.VerifyPassword(user.PasswordHash, req.Password); err != nil {
			httpresp.Unauthorized(c, "invalid password")
			return
		}

		token, err := authservice.Sign(secret, user.ID, user.Username)
		if err != nil {
			httpresp.InternalServerError(c, "failed to sign token")
			return
		}

		httpresp.Success(c, loginResponse{
			Token: token,
			User:  toUserResponse(user),
		})
	}
}

// Me 返回当前登录用户信息。
func Me(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		httpresp.Unauthorized(c, "not authenticated")
		return
	}
	httpresp.Success(c, toUserResponse(user))
}

func toUserResponse(u *models.User) userResponse {
	return userResponse{ID: u.ID, Username: u.Username, Role: u.Role}
}
