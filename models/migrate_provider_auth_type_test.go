package models

import "testing"

// TestDropLegacyProviderAuthType 锁 Stage 20 派生化的 schema 收口：providers.auth_type
// 展示副本列在启动迁移中被物理删除（AutoMigrate 只加不删，gorm:"-" 不会自动清列，
// 不删则历史漂移值永久残留——原则 0.1 删到位，同款见 dropLegacyStatsModelTable）。
func TestDropLegacyProviderAuthType(t *testing.T) {
	db := testChannelDB(t)

	// 前置：模拟存量库——AutoMigrate 对 gorm:"-" 字段不建列，手动补出遗留列。
	if err := db.Exec("ALTER TABLE providers ADD COLUMN auth_type text").Error; err != nil {
		t.Fatalf("模拟存量列: %v", err)
	}
	// 判据用表名字符串：gorm:"-" 字段不在 schema，HasColumn(&Provider{}) 恒 false。
	if !db.Migrator().HasColumn("providers", "auth_type") {
		t.Fatal("前置条件失败：遗留列应存在")
	}

	dropLegacyProviderAuthType(db)
	if db.Migrator().HasColumn("providers", "auth_type") {
		t.Fatal("遗留列 auth_type 应被迁移删除（双写面物理消失，而非仅靠 gorm:\"-\" 不映射）")
	}

	// 幂等：无列再跑不炸（每次启动都会执行）。
	dropLegacyProviderAuthType(db)
}
