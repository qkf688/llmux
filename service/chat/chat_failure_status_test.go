package chat

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

// statusOf 提取错误自带的对外状态码；无状态码错误返回 ok=false。
func statusOf(t *testing.T, err error) (int, bool) {
	t.Helper()
	var sc interface{ StatusCode() int }
	if !errors.As(err, &sc) {
		return 0, false
	}
	return sc.StatusCode(), true
}

// 配置态失败之一：模型根本不存在 → 404（对齐 OpenAI「model not found」口径），
// 客户端不重试；文案带模型名便于定位，不得含内部错误文本。
func TestProvidersWithMetaBymodelsName_ModelNotFound_Returns404(t *testing.T) {
	initChatRecordTestDB(t)

	_, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: "ghost-model"},
		models.ReqMeta{})
	if err == nil {
		t.Fatalf("expected error")
	}
	status, ok := statusOf(t, err)
	if !ok || status != http.StatusNotFound {
		t.Fatalf("status = %d (ok=%v), want 404（模型不存在须回 404 而非 500）", status, ok)
	}
	if !strings.Contains(err.Error(), "ghost-model") {
		t.Fatalf("message = %q, want 含请求的模型名", err.Error())
	}
}

// 配置态失败之二：模型存在但无任何启用关联（没关联/全停用/能力筛光/白名单对不上的
// 共同终态）→ 404 + 指向后台配置的文案。重试不可能变好，500 只会诱发无效重试。
func TestProvidersWithMetaBymodelsName_NoEnabledProvider_Returns404(t *testing.T) {
	initChatRecordTestDB(t)

	model := models.Model{Name: "m-no-provider"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	_, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: model.Name},
		models.ReqMeta{})
	if err == nil {
		t.Fatalf("expected error")
	}
	status, ok := statusOf(t, err)
	if !ok || status != http.StatusNotFound {
		t.Fatalf("status = %d (ok=%v), want 404（无可用供应商属配置态，须 404）", status, ok)
	}
	if !strings.Contains(err.Error(), model.Name) {
		t.Fatalf("message = %q, want 含模型名", err.Error())
	}
	if !strings.Contains(err.Error(), "enabled provider") {
		t.Fatalf("message = %q, want 指向「无启用供应商」的可读原因", err.Error())
	}
}

// 重试穷尽（真实路径）：候选池非空但选路必败，耗尽后须回 502 Bad Gateway
// ——语义是「上游全挂」而非「网关自身故障」，客户端重试行为不变。
func TestBalanceChat_RetryExhausted_Returns502(t *testing.T) {
	initChatRecordTestDB(t)

	model := models.Model{Name: "m-exhaust"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	// 无端点/无凭据的供应商：候选池非空但 Select 必败 → 循环淘汰至穷尽。
	provider := models.Provider{Name: "p-unreachable", Type: "no-such-provider-type"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	enabled := true
	assoc := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "upstream-exhaust",
		Status:        &enabled,
		Weight:        10,
		Priority:      1,
	}
	if err := models.DB.Create(&assoc).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	pwm, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: model.Name},
		models.ReqMeta{})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}

	_, err = BalanceChat(context.Background(), BalanceInput{
		Start:             time.Now(),
		Style:             "openai",
		Before:            Before{Model: model.Name},
		ProvidersWithMeta: *pwm,
		ReqMeta:           models.ReqMeta{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	status, ok := statusOf(t, err)
	if !ok || status != http.StatusBadGateway {
		t.Fatalf("status = %d (ok=%v), want 502（重试穷尽须回 502 而非 500）", status, ok)
	}
}
