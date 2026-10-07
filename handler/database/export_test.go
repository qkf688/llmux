package database

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// isExportSnapshot 判定目录项是不是实现产出的导出临时快照。
//
// 实现命名（export.go 的 snapshotPath）= <库路径>.export-<时间戳>.tmp，例如
// /tmp/x/testutil.db.export-20261008_061500.tmp。抽成函数是为了让「判据能不能命中
// 真实命名」有一条机械控制（见用例内的控制表）——原断言用 `HasSuffix("-export")`，
// 与真实命名的 `.tmp` 后缀永不相等、`HasPrefix("llmux.db")` 在库名归 testutil 唯一名后
// 也一并失效，断言恒真：把清理整个删掉仍全绿（Stage 41 修的就是这个假绿）。
func isExportSnapshot(name string) bool {
	return strings.Contains(name, ".export-") && strings.HasSuffix(name, ".tmp")
}

// TestExportDatabase_TempSnapshotCleanedUp 导出后临时快照文件不残留。
//
// 判据二层：① 控制表锁「判据本身能命中实现真实命名、且不再依赖已失效的库名字面量」；
// ② 目录扫描断无残留（把 export.go 的 defer os.Remove 注释掉必须变红——这正是本用例
// 存在的意义，改前它是恒真的假绿）。
func TestExportDatabase_TempSnapshotCleanedUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, dbPath := openExportFixtureDB(t)
	swapDBPath(t, dbPath, db)

	// 控制表：判据必须命中「实现真实产出的命名」，且不命中历史错配形态。
	//
	// 第一行由实现侧构造器 exportSnapshotPath 产出——实现改名而判据没跟上时这一行立刻红。
	// 缺了它，期望值全部出自测试自己的字面量，改名后判据与期望一起错、目录扫描恒绿，
	// 假绿原样复活（这正是 Stage 41 修的形态；第一版控制表只锚了自己的期望，没锁住它）。
	base := filepath.Base(dbPath)
	controls := []struct {
		name string
		want bool
	}{
		{name: filepath.Base(exportSnapshotPath(dbPath, time.Now())), want: true},
		{name: base + ".export-20261008_061500.tmp", want: true},
		{name: "llmux.db.export-20261008_061500.tmp", want: true},
		{name: base + "-export", want: false}, // 原断言的过滤形态：对真实命名永不命中
		{name: base, want: false},
		{name: base + "-wal", want: false},
	}
	for _, tc := range controls {
		if got := isExportSnapshot(tc.name); got != tc.want {
			t.Fatalf("isExportSnapshot(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}

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
		if isExportSnapshot(e.Name()) {
			t.Fatalf("temp export snapshot left behind: %s", e.Name())
		}
	}
}
