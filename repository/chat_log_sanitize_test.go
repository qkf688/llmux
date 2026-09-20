package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
)

// Stage 16 契约（存量清洗）：启动装配时一次性把存量 chat_logs.request_headers 里的
// 明文敏感头值清洗为 ****（键保留、只改值不删行）；幂等——已清洗行二次执行零改写；
// 坏 JSON 行跳过且不上抛错误（启动不得被阻断）。判据单源 common.MaskSensitiveLogHeadersJSON。

func TestChatLogRepo_SanitizeLegacyRequestHeaders(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	const (
		plaintext   = `{"Authorization":["Bearer sk-legacy"],"Cookie":["session=old"],"X-Custom":["keep"]}`
		maskedForm  = `{"Authorization":["****"],"Cookie":["****"],"X-Custom":["keep"]}`
		badJSON     = `{"Authorization":`
		noSensitive = `{"X-Custom":["keep"]}`
	)
	rows := []models.ChatLog{
		{Name: "m-plain", RequestHeaders: plaintext},
		{Name: "m-masked", RequestHeaders: maskedForm},
		{Name: "m-broken", RequestHeaders: badJSON},
		{Name: "m-empty", RequestHeaders: ""},
		{Name: "m-no-sensitive", RequestHeaders: noSensitive},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	updated, err := repo.SanitizeLegacyRequestHeaders(ctx)
	if err != nil {
		t.Fatalf("SanitizeLegacyRequestHeaders: %v（坏 JSON 行必须跳过而非上抛）", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1（只有明文行需要改写）", updated)
	}

	reload := func(idx int) string {
		var got models.ChatLog
		if err := db.First(&got, rows[idx].ID).Error; err != nil {
			t.Fatalf("reload row %d: %v", idx, err)
		}
		return got.RequestHeaders
	}

	plain := reload(0)
	if strings.Contains(plain, "sk-legacy") || strings.Contains(plain, "session=old") {
		t.Fatalf("明文行未被清洗: %s", plain)
	}
	if !strings.Contains(plain, `"Authorization":["****"]`) || !strings.Contains(plain, `"X-Custom":["keep"]`) {
		t.Fatalf("清洗后键保留/值掩码/非敏感原样契约破坏: %s", plain)
	}
	if got := reload(1); got != maskedForm {
		t.Fatalf("已清洗行被改写（幂等破坏）: %s", got)
	}
	if got := reload(2); got != badJSON {
		t.Fatalf("坏 JSON 行被改写: %s", got)
	}
	if got := reload(3); got != "" {
		t.Fatalf("空行被改写: %q", got)
	}
	if got := reload(4); got != noSensitive {
		t.Fatalf("无敏感头行被改写: %s", got)
	}

	// 幂等：二次执行零改写（清洗随每次启动执行，重复跑不得再动库）。
	updatedAgain, err := repo.SanitizeLegacyRequestHeaders(ctx)
	if err != nil {
		t.Fatalf("二次清洗: %v", err)
	}
	if updatedAgain != 0 {
		t.Fatalf("二次清洗 updated = %d, want 0（幂等）", updatedAgain)
	}
}
