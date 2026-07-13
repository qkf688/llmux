package providerapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atopos31/llmio/handler/testsupport"
	"github.com/gin-gonic/gin"
)

func TestUpdateProvider_NotFoundReturnsEnvelope404(t *testing.T) {
	testsupport.InitTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/providers/999", strings.NewReader(`{"model_endpoint":false}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	UpdateProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 404 {
		t.Fatalf("payload code = %d, want 404, body=%s", payload.Code, w.Body.String())
	}
	if payload.Message != "Provider not found" {
		t.Fatalf("payload message = %q, want %q, body=%s", payload.Message, "Provider not found", w.Body.String())
	}
}
