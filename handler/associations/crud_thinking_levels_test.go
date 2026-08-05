package associations

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

// createModelForThinkingTest 创建一个 model 供 association Create 测试使用。
func createModelForThinkingTest(t *testing.T) {
	t.Helper()
	m := models.Model{Name: "test-model"}
	if err := repository.Default().Model.Create(t.Context(), &m); err != nil {
		t.Fatalf("create model: %v", err)
	}
}

// TestUpdateModelProvider_ThinkingLevelsTriState 覆盖 ThinkingLevels *[]string 三态 JSON 契约：
// 1) 不发字段（JSON null）→ *[]string nil（继承）
// 2) 发 [] → *[]string 指向空切片（不约束）
// 3) 发 ["low","high"] → *[]string 指向非空（override 白名单）
func TestUpdateModelProvider_ThinkingLevelsTriState(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	// 1) override = ["low","high"]
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":["low","high"]}`)
	got := reloadAssoc(t, mp.ID)
	if got.ThinkingLevels == nil {
		t.Fatalf("case 1: ThinkingLevels = nil, want non-nil")
	}
	if len(*got.ThinkingLevels) != 2 || (*got.ThinkingLevels)[0] != "low" || (*got.ThinkingLevels)[1] != "high" {
		t.Fatalf("case 1: ThinkingLevels = %v, want [low high]", *got.ThinkingLevels)
	}

	// 2) 不约束 = []
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":[]}`)
	got = reloadAssoc(t, mp.ID)
	if got.ThinkingLevels == nil {
		t.Fatalf("case 2: ThinkingLevels = nil, want non-nil empty slice")
	}
	if len(*got.ThinkingLevels) != 0 {
		t.Fatalf("case 2: ThinkingLevels len = %d, want 0", len(*got.ThinkingLevels))
	}

	// 3) 继承 = 不发字段（nil）
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192}`)
	got = reloadAssoc(t, mp.ID)
	if got.ThinkingLevels != nil {
		t.Fatalf("case 3: ThinkingLevels = %v, want nil (inherit)", *got.ThinkingLevels)
	}
}

// TestUpdateModelProvider_ThinkingLevelsJSONNullAsInherit：
// 前端发 null → *[]string nil（继承，与不发字段等价）。
func TestUpdateModelProvider_ThinkingLevelsJSONNullAsInherit(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	// 先设非空
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":["low"]}`)
	got := reloadAssoc(t, mp.ID)
	if got.ThinkingLevels == nil || len(*got.ThinkingLevels) != 1 {
		t.Fatalf("setup: ThinkingLevels = %v, want [low]", got.ThinkingLevels)
	}

	// 发 null → 继承（nil）
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":null}`)
	got = reloadAssoc(t, mp.ID)
	if got.ThinkingLevels != nil {
		t.Fatalf("null case: ThinkingLevels = %v, want nil (inherit)", *got.ThinkingLevels)
	}
}

// TestCreateModelProvider_ThinkingLevels 覆盖 Create 路径的 ThinkingLevels 三态。
func TestCreateModelProvider_ThinkingLevels(t *testing.T) {
	testsupport.InitTestDB(t)
	// Create 路径需要 model 存在（handler 检查 model_id）
	createModelForThinkingTest(t)

	tests := []struct {
		name      string
		body      string
		wantNil   bool
		wantLen   int
		wantFirst string
	}{
		{
			name:      "create with override",
			body:      `{"model_id":1,"provider_id":1,"provider_name":"pm1","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"thinking_levels":["medium","max"]}`,
			wantNil:   false,
			wantLen:   2,
			wantFirst: "medium",
		},
		{
			name:    "create with empty (不约束)",
			body:    `{"model_id":1,"provider_id":1,"provider_name":"pm2","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"thinking_levels":[]}`,
			wantNil: false,
			wantLen: 0,
		},
		{
			name:    "create without field (继承)",
			body:    `{"model_id":1,"provider_id":1,"provider_name":"pm3","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/model-providers", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			CreateModelProvider(c)

			if w.Code != 200 {
				t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
			}

			// 从响应中提取 ID 并 reload
			idStr := gjson.Get(w.Body.String(), "data.ID").String()
			id, _ := strconv.ParseUint(idStr, 10, 64)
			got := reloadAssoc(t, uint(id))
			if tt.wantNil {
				if got.ThinkingLevels != nil {
					t.Errorf("ThinkingLevels = %v, want nil", *got.ThinkingLevels)
				}
			} else {
				if got.ThinkingLevels == nil {
					t.Fatalf("ThinkingLevels = nil, want non-nil")
				}
				if len(*got.ThinkingLevels) != tt.wantLen {
					t.Errorf("ThinkingLevels len = %d, want %d", len(*got.ThinkingLevels), tt.wantLen)
				}
				if tt.wantLen > 0 && (*got.ThinkingLevels)[0] != tt.wantFirst {
					t.Errorf("ThinkingLevels[0] = %q, want %q", (*got.ThinkingLevels)[0], tt.wantFirst)
				}
			}
		})
	}
}
