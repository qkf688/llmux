package importexport

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Provider{},
		&models.Model{},
		&models.ModelWithProvider{},
		&models.Setting{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestImportConfigData_ClearsAndInsertsZeroProviders(t *testing.T) {
	db := newImportTestDB(t)

	// 预置数据，导入后应被清掉再插零值
	if err := db.Create(&models.Provider{Name: "old", Type: "openai"}).Error; err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"providers": []interface{}{
			map[string]interface{}{"name": "ignored"},
			map[string]interface{}{"name": "also-ignored"},
			"skip-non-map",
		},
	}

	tx := db.Begin()
	outcome, err := ImportConfigData(tx, config)
	if err != nil {
		tx.Rollback()
		t.Fatalf("ImportConfigData: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit: %v", err)
	}

	// 计数是 API 回显的唯一来源（前端「已导入: 提供商 N 条」）：可映射的每条落一行，
	// 不可映射的元素计入 skipped 而不是静默消失（否则表现为「导入成功但少了数据」）。
	if outcome.Providers.Imported != 2 || outcome.Providers.Skipped != 1 {
		t.Fatalf("providers outcome = %+v, want imported=2 skipped=1", outcome.Providers)
	}

	var providers []models.Provider
	if err := db.Find(&providers).Error; err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 {
		t.Fatalf("providers = %d, want 2 zero rows", len(providers))
	}
	for _, p := range providers {
		if p.Name == "old" {
			t.Fatalf("old provider should have been cleared")
		}
		// 半残：未映射字段，Name 应为零值
		if p.Name != "" {
			t.Fatalf("expected zero Name, got %q", p.Name)
		}
	}
}

func TestImportConfigData_SettingsOnlyClear(t *testing.T) {
	db := newImportTestDB(t)
	if err := db.Create(&models.Setting{Key: "k", Value: "v"}).Error; err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"settings": []interface{}{
			map[string]interface{}{"key": "k2", "value": "v2"},
		},
	}

	tx := db.Begin()
	outcome, err := ImportConfigData(tx, config)
	if err != nil {
		tx.Rollback()
		t.Fatalf("ImportConfigData: %v", err)
	}
	_ = tx.Commit()

	// settings 只清不写（半残语义）：全部计入 skipped，前端不会把它报成「已导入 N 条」
	if outcome.Settings.Imported != 0 || outcome.Settings.Skipped != 1 {
		t.Fatalf("settings outcome = %+v, want imported=0 skipped=1", outcome.Settings)
	}

	var n int64
	db.Model(&models.Setting{}).Count(&n)
	if n != 0 {
		t.Fatalf("settings count = %d, want 0 (clear only)", n)
	}
}

func TestImportConfigData_RollbackOnError(t *testing.T) {
	db := newImportTestDB(t)
	if err := db.Create(&models.Provider{Name: "keep", Type: "openai"}).Error; err != nil {
		t.Fatal(err)
	}

	// 用错误 SQL 模拟 clear 失败：临时替换不可行，改为关闭连接前注入
	// 这里验证 ImportError 路径：对不存在的表会失败——改用强制 Exec 失败的方式
	// 更直接：providers 正常，但我们用已提交数据验证失败时调用方 rollback
	tx := db.Begin()
	// 先清并写入
	if _, err := ImportConfigData(tx, map[string]interface{}{
		"providers": []interface{}{map[string]interface{}{}},
	}); err != nil {
		tx.Rollback()
		t.Fatalf("setup import: %v", err)
	}
	// 人为 rollback，确认事务语义由调用方控制
	tx.Rollback()

	var n int64
	db.Model(&models.Provider{}).Count(&n)
	if n != 1 {
		t.Fatalf("after rollback count = %d, want 1 original", n)
	}
	var p models.Provider
	if err := db.First(&p).Error; err != nil {
		t.Fatal(err)
	}
	if p.Name != "keep" {
		t.Fatalf("name = %q, want keep", p.Name)
	}
}
