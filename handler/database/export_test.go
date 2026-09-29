package database

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// openExportFixtureDB 在 t.TempDir() 下建一个 WAL 模式文件库并迁移最小表集，
// 返回已打开的 gorm.DB 与库文件路径。导出回归依赖真实文件库（WAL 未 checkpoint
// 场景在 :memory: 上不存在），不用 :memory:；WAL 形态由 FileOptions.WAL 承担。
func openExportFixtureDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	return testutil.NewFile(t, testutil.FileOptions{WAL: true}, &models.Setting{})
}

// swapDBPath 临时改写 models 的库路径供 handler 读取，并把测试库挂到 models.DB
// （ExportDatabase 经它执行 VACUUM INTO），返回恢复函数。
func swapDBPath(t *testing.T, path string, db *gorm.DB) {
	t.Helper()
	prev := models.GetDBPath()
	models.SetDBPath(path)
	t.Cleanup(func() { models.SetDBPath(prev) })
	prevDB := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prevDB })
}

// TestExportDatabase_ContainsUncheckpointedWALWrites 复现：WAL 中有未 checkpoint
// 的写入时，导出物必须包含该行。旧实现直接 c.File(主库文件)，WAL 尾部的行不在
// 导出物内（SQLite 写入先进 WAL，checkpoint 才合并进主文件）。
func TestExportDatabase_ContainsUncheckpointedWALWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, dbPath := openExportFixtureDB(t)
	swapDBPath(t, dbPath, db)

	if err := db.Create(&models.Setting{Key: "export_probe", Value: "wal-fresh"}).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	// 断言 WAL 模式确实生效，保证用例构造的是目标场景。
	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Skipf("WAL sidecar not present (%v); journal semantics not exercisable", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/api/system/export-database", nil)

	ExportDatabase(c)

	if rec.Code != 200 {
		t.Fatalf("export status = %d, body: %s", rec.Code, rec.Body.String())
	}

	// 打开导出物，断言种子行存在。
	exported := filepath.Join(t.TempDir(), "exported.db")
	if err := os.WriteFile(exported, rec.Body.Bytes(), 0o644); err != nil {
		t.Fatalf("write exported: %v", err)
	}
	// 打开导出物，断言种子行存在（OpenExisting 只挂清理，不建表不迁移）。
	out := testutil.OpenExisting(t, exported)
	var count int64
	if err := out.Model(&models.Setting{}).Where("key = ?", "export_probe").Count(&count).Error; err != nil {
		t.Fatalf("count probe row in export: %v", err)
	}
	if count != 1 {
		t.Fatalf("exported db missing un-checkpointed WAL row (count = %d)", count)
	}
}

// TestExportDatabase_TempSnapshotCleanedUp 导出后临时快照文件不残留。
func TestExportDatabase_TempSnapshotCleanedUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, dbPath := openExportFixtureDB(t)
	swapDBPath(t, dbPath, db)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/api/system/export-database", nil)

	ExportDatabase(c)

	if rec.Code != 200 {
		t.Fatalf("export status = %d, body: %s", rec.Code, rec.Body.String())
	}
	entries, err := os.ReadDir(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "llmux.db") && strings.HasSuffix(e.Name(), "-export") {
			t.Fatalf("temp export snapshot left behind: %s", e.Name())
		}
	}
}
