package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
)

// Stage 26 契约（存量清洗扩展）：
// ① response_headers（此前从未脱敏）按静态名单一次性掩码；
// ② request_headers 清洗判据动态并入「现存配置自配头名全集」（extraKeys）；
// ③ 两列共用一次游标清洗 + 实际改写后 VACUUM。

func TestChatLogRepo_SanitizeLegacyLogs_ResponseHeadersMasked(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	const (
		// 键序按字母序写（json.Marshal 对 map 键排序输出），否则「掩码后重排」会被误判为有改写。
		plaintext  = `{"Content-Type":["application/json"],"Server":["nginx"],"Set-Cookie":["sid=leak"]}`
		maskedForm = `{"Content-Type":["application/json"],"Server":["nginx"],"Set-Cookie":["****"]}`
	)
	rows := []models.ChatLog{
		{Name: "m-resp-plain", ResponseHeaders: plaintext},
		{Name: "m-resp-clean", ResponseHeaders: maskedForm},
		{Name: "m-resp-empty", ResponseHeaders: ""},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	updated, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("SanitizeLegacyLogSnapshots: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1（只有明文行需要改写）", updated)
	}

	var got models.ChatLog
	if err := db.First(&got, rows[0].ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(got.ResponseHeaders), &parsed); err != nil {
		t.Fatalf("not json: %v (%s)", err, got.ResponseHeaders)
	}
	if parsed["Set-Cookie"][0] != common.SensitiveHeaderMask {
		t.Fatalf("Set-Cookie 未掩码: %s", got.ResponseHeaders)
	}
	if parsed["Content-Type"][0] != "application/json" || parsed["Server"][0] != "nginx" {
		t.Fatalf("非敏感响应头被改动: %s", got.ResponseHeaders)
	}

	// 幂等：二次执行零改写。
	updatedAgain, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("二次清洗: %v", err)
	}
	if updatedAgain != 0 {
		t.Fatalf("二次清洗 updated = %d, want 0（幂等）", updatedAgain)
	}
}

func TestChatLogRepo_SanitizeLegacyLogSnapshots_ExtraKeysCoverCustomHeaders(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	rows := []models.ChatLog{
		{
			Name: "m-custom",
			// 自配凭据头 X-Relay-Token 历史明文；请求头快照（键序字母序，理由同上）。
			RequestHeaders: `{"X-Keep":["keep"],"X-Relay-Token":["relay-sk-old"]}`,
		},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	// 不传 extraKeys：X-Relay-Token 不在固定名单，值保留（现有配置不知道它时洗不到，
	// 这是文档化局限）。
	updated, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("sanitize no-extra: %v", err)
	}
	if updated != 0 {
		t.Fatalf("updated = %d, want 0（无固定名单命中时零改写）", updated)
	}

	// 传入现存配置头名全集：X-Relay-Token 值掩码、键保留、其他头原样。
	updated, err = repo.SanitizeLegacyLogSnapshots(ctx, []string{"x-relay-token"})
	if err != nil {
		t.Fatalf("sanitize with-extra: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}
	var got models.ChatLog
	if err := db.First(&got, rows[0].ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	var parsed http.Header
	if err := json.Unmarshal([]byte(got.RequestHeaders), &parsed); err != nil {
		t.Fatalf("not json: %v (%s)", err, got.RequestHeaders)
	}
	if parsed.Get("X-Relay-Token") != common.SensitiveHeaderMask {
		t.Fatalf("自配凭据头未掩码: %s", got.RequestHeaders)
	}
	if parsed.Get("X-Keep") != "keep" {
		t.Fatalf("非名单头被改动: %s", got.RequestHeaders)
	}

	// 幂等。
	updatedAgain, err := repo.SanitizeLegacyLogSnapshots(ctx, []string{"x-relay-token"})
	if err != nil {
		t.Fatalf("二次清洗: %v", err)
	}
	if updatedAgain != 0 {
		t.Fatalf("二次清洗 updated = %d, want 0（幂等）", updatedAgain)
	}
}

func TestChatLogRepo_SanitizeLegacyLogSnapshots_CoversBothColumns(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	rows := []models.ChatLog{
		{
			Name:            "m-both",
			RequestHeaders:  `{"Authorization":["Bearer sk-a"]}`,
			ResponseHeaders: `{"Set-Cookie":["sid=b"]}`,
		},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	updated, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1（同一行两列都命中只算一行改写）", updated)
	}
	var got models.ChatLog
	if err := db.First(&got, rows[0].ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !json.Valid([]byte(got.ResponseHeaders)) || got.ResponseHeaders == `{"Set-Cookie":["sid=b"]}` {
		t.Fatalf("响应列未被清洗: %s", got.ResponseHeaders)
	}
	if !json.Valid([]byte(got.RequestHeaders)) || got.RequestHeaders == `{"Authorization":["Bearer sk-a"]}` {
		t.Fatalf("请求列未被清洗: %s", got.RequestHeaders)
	}
}
