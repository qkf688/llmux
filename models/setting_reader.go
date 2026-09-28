package models

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

// GetSettingBool 读取一个 bool 类型的设置项。
// 若设置不存在或解析失败，返回 defaultValue。
func GetSettingBool(ctx context.Context, key string, defaultValue bool) bool {
	v, _ := GetSettingBoolE(ctx, key, defaultValue)
	return v
}

// GetSettingInt 读取一个 int 类型的设置项。
// 若设置不存在、解析失败或值小于 minValue，返回 defaultValue。
// minValue 为 0 表示不强制最小值约束（仅校验解析成功）。
func GetSettingInt(ctx context.Context, key string, defaultValue, minValue int) int {
	v, _ := GetSettingIntE(ctx, key, defaultValue, minValue)
	return v
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

// GetSettingBoolE 与 GetSettingBool 同取值语义，但把「读失败」与「未设置」分开
// （Stage 33）：
//   - key 不存在 → (defaultValue, nil)：未设置是有效状态，default 就是配置本身；
//   - 查询失败（DB 不可用 / 请求预算到期等）→ (defaultValue, err)：defaultValue 不可信，
//     调用方必须按 err 显式降级，禁止拿它冒充真配置去执行写动作。
//
// bool 无「存值非法」形态（非 "true" 即 false 是既有既定语义），故 err 只表达查询失败。
func GetSettingBoolE(ctx context.Context, key string, defaultValue bool) (bool, error) {
	if DB == nil {
		return defaultValue, nil
	}
	setting, err := gorm.G[Setting](DB).Where("key = ?", key).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultValue, nil
		}
		return defaultValue, fmt.Errorf("get setting %s: %w", key, err)
	}
	return setting.Value == "true", nil
}

// GetSettingIntE 与 GetSettingInt 同取值语义，区分口径见 GetSettingBoolE。
// 与 bool 不同，int 有「存值非法」形态（解析失败 / 低于 minValue）：存值非法同按
// 读失败处理（err 非 nil）——配置完整性问题不该退默认值静默续行，让调用方按
// 显式降级处置并留痕。
func GetSettingIntE(ctx context.Context, key string, defaultValue, minValue int) (int, error) {
	if DB == nil {
		return defaultValue, nil
	}
	setting, err := gorm.G[Setting](DB).Where("key = ?", key).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultValue, nil
		}
		return defaultValue, fmt.Errorf("get setting %s: %w", key, err)
	}
	value, err := strconv.Atoi(setting.Value)
	if err != nil {
		return defaultValue, fmt.Errorf("get setting %s: invalid int value %q: %w", key, setting.Value, err)
	}
	if value < minValue {
		return defaultValue, fmt.Errorf("get setting %s: value %d below min %d", key, value, minValue)
	}
	return value, nil
}
