package testutil

import (
	"database/sql"
	"testing"

	"gorm.io/gorm"
)

// ConfigureSQLiteForSingleConn configures SQLite for single connection to avoid
// table visibility issues in memory databases during tests.
func ConfigureSQLiteForSingleConn(t *testing.T, db *gorm.DB) *sql.DB {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	// sqlite 内存库在多连接下会出现表不可见问题，测试强制单连接。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return sqlDB
}
