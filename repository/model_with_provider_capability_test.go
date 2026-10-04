package repository

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// 能力继承契约测试：ListEnabledByModelID 的 COALESCE(mwp.<col>, models.<col>) 解析。
//
// 为什么必须测这一层：能力过滤若退化成单表 `WHERE tool_call = true`，
// 「继承且模型本身支持」的关联会被全部误杀——上层表现为请求 404 no enabled provider，
// 但没有任何一条日志说明真因是继承语义被忽略。这组用例锁死 override 与继承的优先级，
// 且每条断言都能区分「override 优先」与「继承优先」两种相反实现。

// newCapTestDB 必须同时建 models 表：能力过滤是 JOIN + COALESCE，
// 只建关联表会让 JOIN 直接报 no such table（与过滤语义无关的假红）。
func newCapTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.New(t, &models.Model{}, &models.ModelWithProvider{}, &models.Provider{})
}

func boolPtr(b bool) *bool { return &b }

// seedCapModel 建带能力真值的 model；assocCaps 中三兄弟留 nil 即继承。
func seedCapModel(t *testing.T, db *gorm.DB, toolCall, structured, image bool, assocCaps []models.ModelWithProvider) uint {
	t.Helper()
	m := models.Model{
		Name:             "cap-model",
		ToolCall:         toolCall,
		StructuredOutput: structured,
		Image:            image,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	on := true
	for i := range assocCaps {
		assocCaps[i].ModelID = m.ID
		assocCaps[i].ProviderID = 1
		// 只在 Status 未显式给值时兜底为启用；显式给 false 的行必须原样保留，
		// 否则本组用例要验的「停用关联被 status 过滤」会被夹具自己抹掉（恒真假绿）。
		if assocCaps[i].Status == nil {
			assocCaps[i].Status = &on
		}
		assocCaps[i].Weight = 100
		assocCaps[i].Priority = 100
		if err := db.Create(&assocCaps[i]).Error; err != nil {
			t.Fatalf("create assoc %d: %v", i, err)
		}
	}
	return m.ID
}

func assocNames(rows []models.ModelWithProvider) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ProviderModel)
	}
	return out
}

// assertSameAssocNames 比对集合（顺序不定，故按集合判）。
func assertSameAssocNames(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("provider_model 集合不匹配: got %v, want %v", got, want)
	}
	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	for _, g := range got {
		if !wantSet[g] {
			t.Fatalf("多出 %q: got %v, want %v", g, got, want)
		}
	}
}

// TestListEnabledByModelID_CapabilityInheritance 模型真值全 true 时，
// 关联侧的 nil=继承模型能力、显式 false=override 为不支持。
func TestListEnabledByModelID_CapabilityInheritance(t *testing.T) {
	ctx := context.Background()
	db := newCapTestDB(t)
	repo := NewModelWithProviderRepo(db)

	modelID := seedCapModel(t, db, true, true, true, []models.ModelWithProvider{
		{ProviderModel: "inherit-all"},
		// 只覆盖掉后两项：tool 仍继承 → tool 过滤命中它
		{ProviderModel: "inherit-tool-only", StructuredOutput: boolPtr(false), Image: boolPtr(false)},
		// 只覆盖掉 tool：另两项仍为 nil → 继承模型 true，仍应命中 structured/image 过滤。
		// 这行同时是「override 按字段独立生效」的判据：只关一项不影响其余两项。
		{ProviderModel: "override-off-tool", ToolCall: boolPtr(false)},
		{ProviderModel: "override-off-all", ToolCall: boolPtr(false), StructuredOutput: boolPtr(false), Image: boolPtr(false)},
	})

	tests := []struct {
		name string
		caps ModelCapabilityFilter
		want []string
	}{
		{"tool_call 过滤", ModelCapabilityFilter{ToolCall: true}, []string{"inherit-all", "inherit-tool-only"}},
		{"structured_output 过滤", ModelCapabilityFilter{StructuredOutput: true}, []string{"inherit-all", "override-off-tool"}},
		{"image 过滤", ModelCapabilityFilter{Image: true}, []string{"inherit-all", "override-off-tool"}},
		{"三项同时过滤", ModelCapabilityFilter{ToolCall: true, StructuredOutput: true, Image: true}, []string{"inherit-all"}},
		{"空过滤=严格匹配关闭，不过滤", ModelCapabilityFilter{}, []string{"inherit-all", "inherit-tool-only", "override-off-tool", "override-off-all"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.ListEnabledByModelID(ctx, modelID, tc.caps)
			if err != nil {
				t.Fatalf("ListEnabledByModelID: %v", err)
			}
			assertSameAssocNames(t, assocNames(got), tc.want)
		})
	}
}

// TestListEnabledByModelID_ModelCapsFalse 模型真值为 false 时，关联侧不写 override
// 就必须被筛掉；写显式 true 才命中（override 可以比模型强）。
func TestListEnabledByModelID_ModelCapsFalse(t *testing.T) {
	ctx := context.Background()
	db := newCapTestDB(t)
	repo := NewModelWithProviderRepo(db)

	modelID := seedCapModel(t, db, false, false, true, []models.ModelWithProvider{
		{ProviderModel: "inherit-all"},
		{ProviderModel: "override-on", ToolCall: boolPtr(true)},
	})

	t.Run("tool_call 过滤命中显式 override", func(t *testing.T) {
		got, err := repo.ListEnabledByModelID(ctx, modelID, ModelCapabilityFilter{ToolCall: true})
		if err != nil {
			t.Fatalf("ListEnabledByModelID: %v", err)
		}
		assertSameAssocNames(t, assocNames(got), []string{"override-on"})
	})

	t.Run("image 过滤命中继承真值", func(t *testing.T) {
		got, err := repo.ListEnabledByModelID(ctx, modelID, ModelCapabilityFilter{Image: true})
		if err != nil {
			t.Fatalf("ListEnabledByModelID: %v", err)
		}
		// 两行 image 都为 nil → 均继承模型的 image=true
		assertSameAssocNames(t, assocNames(got), []string{"inherit-all", "override-on"})
	})
}

// TestListEnabledByModelID_StopDisabledAssociation 回归：状态过滤不能被 JOIN 挤掉。
func TestListEnabledByModelID_StopDisabledAssociation(t *testing.T) {
	ctx := context.Background()
	db := newCapTestDB(t)
	repo := NewModelWithProviderRepo(db)

	off := false
	modelID := seedCapModel(t, db, true, true, true, []models.ModelWithProvider{
		{ProviderModel: "enabled"},
		{ProviderModel: "disabled", Status: &off},
	})

	got, err := repo.ListEnabledByModelID(ctx, modelID, ModelCapabilityFilter{ToolCall: true})
	if err != nil {
		t.Fatalf("ListEnabledByModelID: %v", err)
	}
	assertSameAssocNames(t, assocNames(got), []string{"enabled"})
}
