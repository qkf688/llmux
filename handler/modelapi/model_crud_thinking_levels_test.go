package modelapi

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// createModelForUpdateTest 创建一个 model 供 UpdateModel 测试使用。
func createModelForUpdateTest(t *testing.T) models.Model {
	t.Helper()
	m := models.Model{Name: "test-model-update"}
	if err := repository.Default().Model.Create(t.Context(), &m); err != nil {
		t.Fatalf("create model: %v", err)
	}
	return m
}

func updateModelViaHandler(t *testing.T, id uint, body string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("PUT", "/models/"+strconv.FormatUint(uint64(id), 10), strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModel(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
}

func reloadModel(t *testing.T, id uint) *models.Model {
	t.Helper()
	got, err := repository.Default().Model.Get(t.Context(), id)
	if err != nil {
		t.Fatalf("reload model: %v", err)
	}
	return got
}

// TestUpdateModel_ThinkingLevelsSerialization 回归 Critical：
// map-based UpdateFields 不走 GORM serializer:json，需手动 JSON 序列化。
// 不序列化直接传 []string 会导致 GORM 报 unsupported type 或写入格式错误。
func TestUpdateModel_ThinkingLevelsSerialization(t *testing.T) {
	testsupport.InitTestDB(t)
	m := createModelForUpdateTest(t)

	// 1) 非空白名单
	updateModelViaHandler(t, m.ID, `{"name":"test-model-update","max_retry":3,"time_out":30,"io_log":true,"thinking_levels":["low","high","max"]}`)
	got := reloadModel(t, m.ID)
	if len(got.ThinkingLevels) != 3 {
		t.Fatalf("case 1: ThinkingLevels len = %d, want 3, value=%v", len(got.ThinkingLevels), got.ThinkingLevels)
	}
	if got.ThinkingLevels[0] != "low" || got.ThinkingLevels[1] != "high" || got.ThinkingLevels[2] != "max" {
		t.Fatalf("case 1: ThinkingLevels = %v, want [low high max]", got.ThinkingLevels)
	}

	// 2) 空白名单（不约束）
	updateModelViaHandler(t, m.ID, `{"name":"test-model-update","max_retry":3,"time_out":30,"io_log":true,"thinking_levels":[]}`)
	got = reloadModel(t, m.ID)
	if len(got.ThinkingLevels) != 0 {
		t.Fatalf("case 2: ThinkingLevels len = %d, want 0, value=%v", len(got.ThinkingLevels), got.ThinkingLevels)
	}

	// 3) 不发字段（nil → 空切片语义）
	updateModelViaHandler(t, m.ID, `{"name":"test-model-update","max_retry":3,"time_out":30,"io_log":true}`)
	got = reloadModel(t, m.ID)
	if len(got.ThinkingLevels) != 0 {
		t.Fatalf("case 3: ThinkingLevels len = %d, want 0 (nil→empty), value=%v", len(got.ThinkingLevels), got.ThinkingLevels)
	}

	// 4) 响应中 thinking_levels 字段正确返回
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(m.ID), 10)}}
	c.Request = httptest.NewRequest("PUT", "/models/"+strconv.FormatUint(uint64(m.ID), 10),
		strings.NewReader(`{"name":"test-model-update","max_retry":3,"time_out":30,"io_log":true,"thinking_levels":["medium","xhigh"]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateModel(c)
	if w.Code != 200 {
		t.Fatalf("case 4: status = %d, body=%s", w.Code, w.Body.String())
	}
	levels := gjson.Get(w.Body.String(), "data.thinking_levels").Array()
	if len(levels) != 2 {
		t.Fatalf("case 4: response thinking_levels len = %d, want 2", len(levels))
	}
	if levels[0].String() != "medium" || levels[1].String() != "xhigh" {
		t.Fatalf("case 4: response thinking_levels = %v, want [medium xhigh]", levels)
	}
}
