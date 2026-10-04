package autoassoc

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

// TestNewDefaultAssociation_CapabilitiesInherit 锁定自动关联默认能力为「继承」。
//
// 此前此处硬编码 ToolCall=true / StructuredOutput=false / Image=false：等于把
// 「模型固有能力」复制到每条关联上，新关联一落地就成了 override，与前端创建弹窗的
// 默认全 true 口径也各说各话。多数第三方与官方能力一致，默认就该是 nil（继承）。
func TestNewDefaultAssociation_CapabilitiesInherit(t *testing.T) {
	assoc := NewDefaultAssociation(1, 2, "pm", DefaultWeightFallback, DefaultPriorityFallback)

	if assoc.ToolCall != nil {
		t.Errorf("ToolCall = %v, want nil（继承 Model.ToolCall）", *assoc.ToolCall)
	}
	if assoc.StructuredOutput != nil {
		t.Errorf("StructuredOutput = %v, want nil（继承 Model.StructuredOutput）", *assoc.StructuredOutput)
	}
	if assoc.Image != nil {
		t.Errorf("Image = %v, want nil（继承 Model.Image）", *assoc.Image)
	}
}

// TestNewDefaultAssociation_NonCapabilityFieldsUnchanged 回归：本次只动能力三态，
// 其余默认值不得被顺手改掉（status 默认启用、weight/priority 透传、with_header 默认关）。
func TestNewDefaultAssociation_NonCapabilityFieldsUnchanged(t *testing.T) {
	assoc := NewDefaultAssociation(7, 9, "gpt-x", 55, 66)

	if assoc.ModelID != 7 || assoc.ProviderID != 9 || assoc.ProviderModel != "gpt-x" {
		t.Errorf("标识字段被改动: %+v", assoc)
	}
	if assoc.Status == nil || !*assoc.Status {
		t.Errorf("Status = %v, want true（自动关联默认启用）", assoc.Status)
	}
	if assoc.WithHeader == nil || *assoc.WithHeader {
		t.Errorf("WithHeader = %v, want false", assoc.WithHeader)
	}
	if assoc.Weight != 55 || assoc.Priority != 66 {
		t.Errorf("Weight/Priority = %d/%d, want 55/66", assoc.Weight, assoc.Priority)
	}
	var _ models.ModelWithProvider = assoc
}
