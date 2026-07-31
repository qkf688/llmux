package virtualmodels

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
)

// 这些回归测试锁定 #35 的行为变更：
// 非法路径 id 原先由 parseIDParam 静默降级为 0（GET 返回空列表 code=200，
// Update/Delete 走 not found code=404）；删除 parseIDParam 改用
// httpx.ParseUintParamAllowZero 后，非法 id 直接返回业务 code=400。
// httpresp.BadRequest 写 HTTP 200 + envelope Code=400，故两层都校验。

func assertInvalidIDReturns400(t *testing.T, invoke func(c *gin.Context), method, path string, body string) {
	t.Helper()

	testsupport.InitTestDB(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "not-a-number"}}

	invoke(c)

	if w.Code != 200 {
		t.Fatalf("HTTP status = %d, want 200 (envelope 语义), body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 400 {
		t.Fatalf("envelope code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}

func TestGetVirtualModelMappings_InvalidIDReturns400(t *testing.T) {
	assertInvalidIDReturns400(t, GetVirtualModelMappings, "GET", "/virtual-models/not-a-number/mappings", "")
}

func TestUpdateVirtualModel_InvalidIDReturns400(t *testing.T) {
	assertInvalidIDReturns400(t, UpdateVirtualModel, "PUT", "/virtual-models/not-a-number", `{}`)
}

func TestDeleteVirtualModel_InvalidIDReturns400(t *testing.T) {
	assertInvalidIDReturns400(t, DeleteVirtualModel, "DELETE", "/virtual-models/not-a-number", "")
}

func TestGetVirtualModelStats_InvalidIDReturns400(t *testing.T) {
	assertInvalidIDReturns400(t, GetVirtualModelStats, "GET", "/virtual-models/not-a-number/stats", "")
}
