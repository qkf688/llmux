package models

import (
	"context"
	"strconv"

	"gorm.io/gorm"
)

// GetSettingBool 读取一个 bool 类型的设置项。
// 若设置不存在或解析失败，返回 defaultValue。
func GetSettingBool(ctx context.Context, key string, defaultValue bool) bool {
	if DB == nil {
		return defaultValue
	}
	setting, err := gorm.G[Setting](DB).Where("key = ?", key).First(ctx)
	if err != nil {
		return defaultValue
	}
	return setting.Value == "true"
}

// GetSettingInt 读取一个 int 类型的设置项。
// 若设置不存在、解析失败或值小于 minValue，返回 defaultValue。
// minValue 为 0 表示不强制最小值约束（仅校验解析成功）。
func GetSettingInt(ctx context.Context, key string, defaultValue, minValue int) int {
	if DB == nil {
		return defaultValue
	}
	setting, err := gorm.G[Setting](DB).Where("key = ?", key).First(ctx)
	if err != nil {
		return defaultValue
	}
	value, err := strconv.Atoi(setting.Value)
	if err != nil || value < minValue {
		return defaultValue
	}
	return value
}

// GetSettingString 读取一个 string 类型的设置项。
// 若设置不存在，返回 defaultValue。
func GetSettingString(ctx context.Context, key string, defaultValue string) string {
	if DB == nil {
		return defaultValue
	}
	setting, err := gorm.G[Setting](DB).Where("key = ?", key).First(ctx)
	if err != nil {
		return defaultValue
	}
	return setting.Value
}
