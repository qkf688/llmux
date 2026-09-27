package httpresp

import (
	"errors"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/common"
)

// Stage 26 slog path 截断测试 fixture：path 本身超 4 KiB（该写点只记 URL.Path，
// 不含 query，长串必须放进 path 段）+ 上限常量别名（单源 common.SlogPathMaxBytes）。

const slogPathLimit = common.SlogPathMaxBytes

func ginTestContextWithLongPath(w *httptest.ResponseRecorder) (*gin.Context, *httptest.ResponseRecorder) {
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/bomb/"+strings.Repeat("x", 8192), nil)
	return c, w
}

func errLongPathFixture() error { return errors.New("boom") }
