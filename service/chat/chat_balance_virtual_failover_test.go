package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/virtualmodel"
)

// 某个真实模型的候选池穷尽（有候选但全部不可用）时，虚拟路径必须继续尝试下一个真实模型，
// 而不是把穷尽当作整体失败——这是虚拟模型故障转移的核心语义。
func TestBalanceChatVirtual_PoolExhausted_TriesNextRealModel(t *testing.T) {
	virtualmodel.ResetSingletonForTest()
	initChatRecordTestDB(t)

	enabled := true

	virtual := models.VirtualModel{Name: "vm-failover", Strategy: "priority", Enabled: &enabled}
	if err := models.DB.Create(&virtual).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	first := models.Model{Name: "first-real", MaxRetry: 2, TimeOut: 10}
	second := models.Model{Name: "second-real", MaxRetry: 2, TimeOut: 10}
	for _, m := range []*models.Model{&first, &second} {
		if err := models.DB.Create(m).Error; err != nil {
			t.Fatalf("create real model %q: %v", m.Name, err)
		}
	}

	// type 未注册 → providers.New 必然失败，循环淘汰该候选后报告穷尽。
	provider := models.Provider{Name: "bad-provider", Type: "no-such-provider-type"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	// 只给第一个真实模型挂关联：它的候选池非空但不可用；第二个无关联，会写 skip 日志。
	assoc := models.ModelWithProvider{
		ModelID:       first.ID,
		ProviderID:    provider.ID,
		ProviderModel: "first-upstream",
		Status:        &enabled,
		Weight:        10,
		Priority:      1,
	}
	if err := models.DB.Create(&assoc).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	_, err := balanceChatVirtual(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  "openai",
		Before: Before{Model: virtual.Name},
		ProvidersWithMeta: ProvidersWithMeta{
			IsVirtualModel:    true,
			VirtualModelID:    virtual.ID,
			VirtualModelName:  virtual.Name,
			VirtualStrategy:   virtual.Strategy,
			MaxRetry:          2,
			TimeOut:           10,
			OrderedRealModels: []virtualmodel.OrderedRealModel{{Model: first}, {Model: second}},
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "all real models exhausted") {
		t.Fatalf("error = %q, want contains %q", err.Error(), "all real models exhausted")
	}

	// 第二个真实模型的 skip 日志证明：第一个模型候选池穷尽后外层循环确实推进到了下一个模型。
	var logs []models.ChatLog
	if err := models.DB.Where("real_model_name = ?", second.Name).Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("logs for %q = %d, want 1 (outer loop did not advance past exhausted pool)", second.Name, len(logs))
	}
}
