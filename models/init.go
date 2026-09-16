package models

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Init 打开数据库并执行迁移与种子/兼容回填。
func Init(ctx context.Context, path string) {
	if err := ensureDBFile(path); err != nil {
		panic(err)
	}
	// busy_timeout + WAL：凭据健康异步写与前台读并发时降低 database is locked 概率。
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	// logger 走 slog 桥接且常开参数脱敏（Stage 1：默认 logger 会把失败语句参数值——
	// 含 api_keys/credentials 密文与 hash——整条打进 stdout），配置单源见 gormConfig。
	// 注意：此处捕获调用时刻的 slog.Default()，main 若将来自定义 handler 须在
	// models.Init 之前完成装配。
	db, err := gorm.Open(sqlite.Open(dsn), gormConfig(slog.Default()))
	if err != nil {
		panic(err)
	}
	DB = db
	migrate(ctx)
	seed(ctx)
}

// Close 关闭数据库连接，供进程优雅关闭时在**排空后台写库任务之后**调用。
// 提前调用会让仍在排空的任务撞上「数据库已关闭」。
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

var dbPath string

// SetDBPath 设置数据库路径
func SetDBPath(path string) {
	dbPath = path
}

// GetDBPath 获取数据库路径
func GetDBPath() string {
	return dbPath
}

func ensureDBFile(path string) error {
	// 保存数据库路径
	SetDBPath(path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
