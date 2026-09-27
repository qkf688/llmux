package repository

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// Stage 26 回归（risk-review C1 同款口径扩展到响应列）：SanitizeLegacyLogSnapshots
// 清洗后主库文件的物理字节不得再含明文——UPDATE 旧值驻留页 freeblock/slack
// （secure_delete 默认关），必须 VACUUM 重建库文件，否则整库导出面可静态恢复明文。
// 显式关库再扫主文件（Windows 不 Close 还会让 t.TempDir 清理失败把用例判红）。
func TestChatLogRepo_SanitizeLegacyLogSnapshots_PhysicalBytesErased(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sanitize26-bytes.db")
	db, err := gorm.Open(sqlite.Open(dbPath))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.ChatLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewChatLogRepo(db)

	const (
		reqSecret  = "sk-legacy-REQ-MARKER"
		respSecret = "sid-legacy-RESP-MARKER"
	)
	if err := db.Create(&models.ChatLog{
		Name:            "m-both",
		RequestHeaders:  `{"Authorization":["Bearer ` + reqSecret + `"],"X-Keep":["keep"]}`,
		ResponseHeaders: `{"Set-Cookie":[` + `"` + respSecret + `"]}`,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := repo.SanitizeLegacyLogSnapshots(ctx, nil)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}

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
	for _, secret := range []string{reqSecret, respSecret} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("清洗后主库文件物理字节仍含明文 %q（VACUUM 未生效，导出面可恢复）: %s", secret, dbPath)
		}
	}
	if !bytes.Contains(raw, []byte("X-Keep")) {
		t.Fatalf("VACUUM 后活数据丢失: %s", dbPath)
	}
}
