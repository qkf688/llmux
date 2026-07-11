package models

import (
	"context"
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
	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		panic(err)
	}
	DB = db
	migrate(ctx)
	seed(ctx)
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