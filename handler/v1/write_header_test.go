package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/consts"
)

func init() { gin.SetMode(gin.TestMode) }

// Stage 3：响应头 X-Request-Id 唯一归属网关。上游（Azure/OpenAI 常见）自带同名头时
// 必须改名 X-Upstream-Request-Id 回传——否则客户端在一个头里看到 [网关号, 供应商号]
// 两值，取哪个全凭 SDK 实现，报障编号失去意义；改名后向供应商报障仍拿得到原号。
func TestWriteHeader_UpstreamRequestIDRenamedNotAppended(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	up := http.Header{}
	up.Set(consts.HeaderRequestID, "vendor-xyz")
	up.Set("Content-Type", "application/json")

	writeHeader(c, context.Background(), false, up)

	if got := w.Header().Values(consts.HeaderRequestID); len(got) != 0 {
		t.Errorf("网关命名空间被上游值占用: %v", got)
	}
	if got := w.Header().Get("X-Upstream-Request-Id"); got != "vendor-xyz" {
		t.Errorf("上游编号应改名为 X-Upstream-Request-Id, got %q", got)
	}
}
