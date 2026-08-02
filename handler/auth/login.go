package auth

import (
	"net/http"

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

// Login 校验密码并签发 JWT。单管理员场景，username 隐式为 admin。
func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "password is required")
		return
	}

	repo := repos().User
	user, err := repo.FindByUsername(c.Request.Context(), authservice.AdminUsername)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "invalid password")
		return
	}

	if err := authservice.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "invalid password")
		return
	}

	token, err := authservice.Sign(jwtSecret, user.ID, user.Username)
	if err != nil {
		httpresp.InternalServerError(c, "failed to sign token")
		return
	}

	httpresp.Success(c, loginResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

// Me 返回当前登录用户信息。
func Me(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusUnauthorized, http.StatusUnauthorized, "not authenticated")
		return
	}
	httpresp.Success(c, toUserResponse(user))
}

func toUserResponse(u *models.User) userResponse {
	return userResponse{ID: u.ID, Username: u.Username, Role: u.Role}
}
