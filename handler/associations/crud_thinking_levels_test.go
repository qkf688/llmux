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
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":["low","high"]}`)
	got := reloadAssoc(t, mp.ID)
	if got.ThinkingLevels == nil {
		t.Fatalf("case 1: ThinkingLevels = nil, want non-nil")
	}
	if len(*got.ThinkingLevels) != 2 || (*got.ThinkingLevels)[0] != "low" || (*got.ThinkingLevels)[1] != "high" {
		t.Fatalf("case 1: ThinkingLevels = %v, want [low high]", *got.ThinkingLevels)
	}

	// 2) 不约束 = []
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":[]}`)
	got = reloadAssoc(t, mp.ID)
	if got.ThinkingLevels == nil {
		t.Fatalf("case 2: ThinkingLevels = nil, want non-nil empty slice")
	}
	if len(*got.ThinkingLevels) != 0 {
		t.Fatalf("case 2: ThinkingLevels len = %d, want 0", len(*got.ThinkingLevels))
	}

	// 3) 继承 = 不发字段（nil）
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192}`)
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
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":["low"]}`)
	got := reloadAssoc(t, mp.ID)
	if got.ThinkingLevels == nil || len(*got.ThinkingLevels) != 1 {
		t.Fatalf("setup: ThinkingLevels = %v, want [low]", got.ThinkingLevels)
	}

	// 发 null → 继承（nil）
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"thinking_levels":null}`)
	got = reloadAssoc(t, mp.ID)
	if got.ThinkingLevels != nil {
		t.Fatalf("null case: ThinkingLevels = %v, want nil (inherit)", *got.ThinkingLevels)
	}
}

// TestCreateModelProvider_ThinkingLevels 覆盖 Create 路径的 ThinkingLevels 三态。
func TestCreateModelProvider_ThinkingLevels(t *testing.T) {
	testsupport.InitTestDB(t)
	// Create 路径校验 model + provider 存在
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	tests := []struct {
		name      string
		body      string
		wantNil   bool
		wantLen   int
		wantFirst string
	}{
		{
			name:      "create with override",
			body:      `{"model_id":1,"provider_id":1,"provider_model":"pm1","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"thinking_levels":["medium","max"]}`,
			wantNil:   false,
			wantLen:   2,
			wantFirst: "medium",
		},
		{
			name:    "create with empty (不约束)",
			body:    `{"model_id":1,"provider_id":1,"provider_model":"pm2","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"thinking_levels":[]}`,
			wantNil: false,
			wantLen: 0,
		},
		{
			name:    "create without field (继承)",
			body:    `{"model_id":1,"provider_id":1,"provider_model":"pm3","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10}`,
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

// TestModelProviderResponse_ThinkingLevelsShape 回归 Critical：
// 关联响应里 ThinkingLevels 键必须**始终存在**，继承态为 JSON null 而非整字段省略。
//
// 键缺失会让前端 `association.ThinkingLevels === null` 恒为 false（undefined !== null），
// 编辑弹窗因此恒判"自定义"，并在保存时把"继承"静默改写成"显式不约束"（[]），
// 使 ClampReasoningEffort 的白名单钳制失效——是数据破坏，不只是显示错。
//
// 断言用 Exists() 区分"键存在且为 null"与"键缺失"：若只断取值为空串，
// 两种情况都会通过，测不出该 bug。这也是原有测试全走 repository 读 DB、
// 绕过序列化层，从而漏掉此 bug 的原因——契约断层只在响应体上可见。
func TestModelProviderResponse_ThinkingLevelsShape(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	const baseFields = `"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192`

	tests := []struct {
		name     string
		body     string
		wantType gjson.Type
		wantRaw  string
	}{
		{
			name:     "继承（请求不发字段）→ 响应键存在且为 null",
			body:     `{` + baseFields + `}`,
			wantType: gjson.Null,
			wantRaw:  "null",
		},
		{
			name:     "override → 响应为数组",
			body:     `{` + baseFields + `,"thinking_levels":["low","high"]}`,
			wantType: gjson.JSON,
			wantRaw:  `["low","high"]`,
		},
		{
			name:     "显式不约束 → 响应为空数组",
			body:     `{` + baseFields + `,"thinking_levels":[]}`,
			wantType: gjson.JSON,
			wantRaw:  `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update 的单个对象响应
			updateBody := updateAssocViaHandler(t, mp.ID, tt.body)
			assertThinkingLevelsShape(t, updateBody, "data.ThinkingLevels", tt.wantType, tt.wantRaw)

			// List 的列表响应（前端编辑弹窗回填读的就是这一条）
			listBody := listAssocsViaHandler(t, 1)
			assertThinkingLevelsShape(t, listBody, "data.0.ThinkingLevels", tt.wantType, tt.wantRaw)
		})
	}
}

// assertThinkingLevelsShape 断言响应体指定路径上的 ThinkingLevels 键存在，且类型与原始值符合预期。
func assertThinkingLevelsShape(t *testing.T, body, path string, wantType gjson.Type, wantRaw string) {
	t.Helper()
	got := gjson.Get(body, path)
	if !got.Exists() {
		t.Fatalf("%s 键缺失（继承态被 omitempty 省略会导致前端恒判自定义并写坏数据），body=%s", path, body)
	}
	if got.Type != wantType {
		t.Fatalf("%s type = %v, want %v, raw=%s", path, got.Type, wantType, got.Raw)
	}
	if got.Raw != wantRaw {
		t.Fatalf("%s raw = %s, want %s", path, got.Raw, wantRaw)
	}
}
