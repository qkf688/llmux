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
// Stage 5 的隐含兼容前提：网关**从不**把上游状态码透传给调用方。上游回 201 的兼容网关
// 现在被判成功（以前判失败），若哪天有人把上游状态码搬到响应上，严格判 `status == 200`
// 的 SDK 会全线报错——而这条前提此前没有任何测试锁定。成功路径的状态码由 gin 首次写出
// 时定为 200，writeHeader 只搬头不写码，这里把「只搬头」钉住。
func TestWriteHeader_NeverEmitsUpstreamStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	up := http.Header{}
	up.Set("Content-Type", "application/json")

	writeHeader(c, context.Background(), false, up)
	_, _ = c.Writer.Write([]byte(`{"id":"chatcmpl-1"}`))

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("客户端状态码 = %d, want 200（上游 201 不得透传，否则严格判 200 的 SDK 全炸）", got)
	}
}

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
