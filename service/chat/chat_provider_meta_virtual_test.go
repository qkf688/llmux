package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/virtualmodel"
)

func TestProvidersWithMetaBymodelsName_VirtualModel_FirstRealModelNoProvider_DoesNotError(t *testing.T) {
	virtualmodel.ResetSingletonForTest()
	initChatRecordTestDB(t)

	enabled := true
	disabled := false

	provider := models.Provider{Name: "p1", Type: "openai", Config: "{}"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	realModel1 := models.Model{Name: "MiniMax-M2"}
	if err := models.DB.Create(&realModel1).Error; err != nil {
		t.Fatalf("create real model 1: %v", err)
	}
	realModel2 := models.Model{Name: "Real-OK"}
	if err := models.DB.Create(&realModel2).Error; err != nil {
		t.Fatalf("create real model 2: %v", err)
	}

	if err := models.DB.Create(&models.ModelWithProvider{
		ModelID:       realModel1.ID,
		ProviderID:    provider.ID,
		ProviderModel: "minimax-m2",
		Status:        &disabled,
		Weight:        1,
		Priority:      1,
	}).Error; err != nil {
		t.Fatalf("create disabled model provider: %v", err)
	}
	if err := models.DB.Create(&models.ModelWithProvider{
		ModelID:       realModel2.ID,
		ProviderID:    provider.ID,
		ProviderModel: "real-ok",
		Status:        &enabled,
		Weight:        1,
		Priority:      1,
	}).Error; err != nil {
		t.Fatalf("create enabled model provider: %v", err)
	}

	virtual := models.VirtualModel{Name: "vm1", Strategy: "round_robin", Enabled: &enabled}
	if err := models.DB.Create(&virtual).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	if err := models.DB.Create(&models.VirtualModelMapping{
		VirtualModelID: virtual.ID,
		RealModelID:    realModel1.ID,
		Enabled:        &enabled,
	}).Error; err != nil {
		t.Fatalf("create mapping 1: %v", err)
	}
	if err := models.DB.Create(&models.VirtualModelMapping{
		VirtualModelID: virtual.ID,
		RealModelID:    realModel2.ID,
		Enabled:        &enabled,
	}).Error; err != nil {
		t.Fatalf("create mapping 2: %v", err)
	}

	got, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: virtual.Name})
	if err != nil {
		t.Fatalf("ProvidersWithMetaBymodelsName error: %v", err)
	}
	if !got.IsVirtualModel {
		t.Fatalf("IsVirtualModel = false, want true")
	}
	if got.VirtualModelName != virtual.Name {
		t.Fatalf("VirtualModelName = %q, want %q", got.VirtualModelName, virtual.Name)
	}
	if len(got.OrderedRealModels) != 2 {
		t.Fatalf("OrderedRealModels length = %d, want 2", len(got.OrderedRealModels))
	}
	if got.OrderedRealModels[0].Model.Name != "MiniMax-M2" {
		t.Fatalf("first ordered real model = %q, want %q", got.OrderedRealModels[0].Model.Name, "MiniMax-M2")
	}
}
