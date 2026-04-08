package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/virtualmodel"
)

func TestBalanceChatVirtual_NoProvidersForRealModel_WritesSkipLog(t *testing.T) {
	virtualmodel.ResetSingletonForTest()
	initChatRecordTestDB(t)

	enabled := true

	virtual := models.VirtualModel{Name: "vm-skip-log", Strategy: "priority", Enabled: &enabled}
	if err := models.DB.Create(&virtual).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	realModel := models.Model{Name: "MiniMax-M2", MaxRetry: 1, TimeOut: 10}
	if err := models.DB.Create(&realModel).Error; err != nil {
		t.Fatalf("create real model: %v", err)
	}

	pwm := ProvidersWithMeta{
		IsVirtualModel:   true,
		VirtualModelID:   virtual.ID,
		VirtualModelName: virtual.Name,
		VirtualStrategy:  virtual.Strategy,
		MaxRetry:         1,
		TimeOut:          10,
		IOLog:            false,
		OrderedRealModels: []virtualmodel.OrderedRealModel{
			{Model: realModel},
		},
	}

	_, _, _, err := balanceChatVirtual(context.Background(), time.Now(), "openai", Before{Model: virtual.Name}, pwm, models.ReqMeta{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var logs []models.ChatLog
	if err := models.DB.Where("name = ?", virtual.Name).Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("logs length = %d, want 1", len(logs))
	}
	if logs[0].Status != "error" {
		t.Fatalf("log status = %q, want %q", logs[0].Status, "error")
	}
	if logs[0].ProviderModel != realModel.Name {
		t.Fatalf("ProviderModel = %q, want %q", logs[0].ProviderModel, realModel.Name)
	}
	if !strings.Contains(logs[0].Error, "no enabled providers") {
		t.Fatalf("Error = %q, want contains %q", logs[0].Error, "no enabled providers")
	}
}
