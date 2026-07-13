package testsupport

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// NewTestContext 构造 gin 测试上下文（无 body）。
func NewTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}
