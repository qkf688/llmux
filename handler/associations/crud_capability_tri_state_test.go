package associations

import (
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/tidwall/gjson"
)

// 关联侧能力三态契约：nil=继承 Model 真值，true/false=override。
//
// 关键风险点：Update 必须能把 override 清回继承（写 NULL）。若三兄弟留在 struct
// Updates 里，nil 指针会被 GORM 当零值跳过，override 一旦写上就再也清不掉——
// 而这正是本次改动要提供的核心能力（多数第三方无需标注，少数要能单独覆盖回来）。

// TestCreateModelProvider_CapabilitiesDefaultToInherit 建关联不发能力字段时必须落 NULL。
func TestCreateModelProvider_CapabilitiesDefaultToInherit(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	body := createAssocViaHandler(t, `{"model_id":1,"provider_id":1,"provider_model":"pm-inherit",`+
		`"with_header":false,"weight":10,"priority":10}`)
	id := uint(gjson.Get(body, "data.ID").Uint())

	got := reloadAssoc(t, id)
	if got.ToolCall != nil {
		t.Errorf("ToolCall = %v, want nil（继承）", *got.ToolCall)
	}
	if got.StructuredOutput != nil {
		t.Errorf("StructuredOutput = %v, want nil（继承）", *got.StructuredOutput)
	}
	if got.Image != nil {
		t.Errorf("Image = %v, want nil（继承）", *got.Image)
	}
}

// TestUpdateModelProvider_CapabilitiesTriState 覆盖 Update 的三态往返：
// nil → true/false（写 override）→ nil（清回继承）。
func TestUpdateModelProvider_CapabilitiesTriState(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	// 1) 发显式值 → 落 override
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm",`+
		`"tool_call":true,"structured_output":false,"image":true,"weight":10,"priority":10}`)
	got := reloadAssoc(t, mp.ID)
	if got.ToolCall == nil || !*got.ToolCall {
		t.Fatalf("case 1: ToolCall = %v, want true", got.ToolCall)
	}
	if got.StructuredOutput == nil || *got.StructuredOutput {
		t.Fatalf("case 1: StructuredOutput = %v, want false", got.StructuredOutput)
	}
	if got.Image == nil || !*got.Image {
		t.Fatalf("case 1: Image = %v, want true", got.Image)
	}

	// 2) 不发能力字段 → 显式清回继承（写 NULL），这是本次改动的核心能力
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm",`+
		`"weight":10,"priority":10}`)
	got = reloadAssoc(t, mp.ID)
	if got.ToolCall != nil {
		t.Errorf("case 2: ToolCall = %v, want nil（已清回继承）", *got.ToolCall)
	}
	if got.StructuredOutput != nil {
		t.Errorf("case 2: StructuredOutput = %v, want nil（已清回继承）", *got.StructuredOutput)
	}
	if got.Image != nil {
		t.Errorf("case 2: Image = %v, want nil（已清回继承）", *got.Image)
	}
}

// TestUpdateModelProvider_CapabilitiesPartialUpdatePreservesOthers 清能力字段时
// 不得顺手清掉其他字段（回归 GORM struct/map 两段更新的分界）。
func TestUpdateModelProvider_CapabilitiesPartialUpdatePreservesOthers(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	before := reloadAssoc(t, mp.ID)
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm",`+
		`"weight":10,"priority":10}`)

	after := reloadAssoc(t, mp.ID)
	if after.MaxTokens == nil || *after.MaxTokens != *before.MaxTokens {
		t.Errorf("MaxTokens 被改动: before=%v after=%v", before.MaxTokens, after.MaxTokens)
	}
	if len(after.CustomerHeaders) != len(before.CustomerHeaders) {
		t.Errorf("CustomerHeaders 被改动: before=%v after=%v", before.CustomerHeaders, after.CustomerHeaders)
	}
	if after.Status == nil || *after.Status != *before.Status {
		t.Errorf("Status 被改动: before=%v after=%v", before.Status, after.Status)
	}
}
