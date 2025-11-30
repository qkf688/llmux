package models

import (
	"context"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(ctx context.Context, path string) {
	if err := ensureDBFile(path); err != nil {
		panic(err)
	}
	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		panic(err)
	}
	DB = db
	if err := db.AutoMigrate(
		&Provider{},
		&Model{},
		&ModelWithProvider{},
		&ChatLog{},
		&ChatIO{},
		&Setting{},
	); err != nil {
		panic(err)
	}
	// 兼容性考虑
	if _, err := gorm.G[ModelWithProvider](DB).Where("status IS NULL").Update(ctx, "status", true); err != nil {
		panic(err)
	}
	if _, err := gorm.G[ModelWithProvider](DB).Where("customer_headers IS NULL").Updates(ctx, ModelWithProvider{
		CustomerHeaders: map[string]string{},
	}); err != nil {
		panic(err)
	}
	// 初始化默认设置
	initDefaultSettings(ctx)
}

// initDefaultSettings 初始化默认设置
func initDefaultSettings(ctx context.Context) {
	defaultSettings := []Setting{
		{Key: SettingKeyStrictCapabilityMatch, Value: "false"}, // 默认关闭严格能力匹配
		{Key: SettingKeyAutoWeightDecay, Value: "false"},       // 默认关闭自动权重衰减
		{Key: SettingKeyAutoWeightDecayDefault, Value: "100"},  // 默认权重值100
		{Key: SettingKeyAutoWeightDecayStep, Value: "1"},       // 默认每次失败减少1
	}

	for _, setting := range defaultSettings {
		// 如果设置不存在则创建
		count, err := gorm.G[Setting](DB).Where("key = ?", setting.Key).Count(ctx, "id")
		if err != nil {
			panic(err)
		}
		if count == 0 {
			if err := gorm.G[Setting](DB).Create(ctx, &setting); err != nil {
				panic(err)
			}
		}
	}
}

func ensureDBFile(path string) error {
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
