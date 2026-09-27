package repository

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
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

	updated, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
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
	updatedAgain, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("二次清洗: %v", err)
	}
	if updatedAgain != 0 {
		t.Fatalf("二次清洗 updated = %d, want 0（幂等）", updatedAgain)
	}
}

// TestChatLogRepo_SanitizeLegacyRequestHeaders_PhysicalBytesErased risk-review C1 回归：
// 清洗后主库文件的物理字节不得再含明文凭据——SQLite UPDATE 不清零被替换的旧记录
// （字节驻留页 freeblock/slack，secure_delete 默认关），必须经 VACUUM 重建库文件；
// 否则整库导出面（GET /api/system/export-database 的 c.File 原样下载）可从静态字节
// 恢复明文，「明文档案已消灭」的声称只在逻辑行成立。
func TestChatLogRepo_SanitizeLegacyRequestHeaders_PhysicalBytesErased(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sanitize-bytes.db")
	db, err := gorm.Open(sqlite.Open(dbPath))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.ChatLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewChatLogRepo(db)

	const secret = "sk-legacy-PLAINTEXT-MARKER"
	plaintext := `{"Authorization":["Bearer ` + secret + `"],"X-Custom":["keep"]}`
	if err := db.Create(&models.ChatLog{Name: "m-plain", RequestHeaders: plaintext}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}

	// WAL/journal 残段在 close 时才合并处置——必须显式关库再扫主文件字节
	// （Windows 上不 Close 还会让 t.TempDir 清理失败把用例判红）。
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db file: %v", err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatalf("清洗后主库文件物理字节仍含明文凭据（VACUUM 未生效，导出面可恢复）: %s", dbPath)
	}
	if !bytes.Contains(raw, []byte("X-Custom")) {
		t.Fatalf("VACUUM 后活数据丢失: %s", dbPath)
	}
}
