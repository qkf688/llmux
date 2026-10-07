package models

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// seed 建默认设置行。
//
// 只做「键不存在才建」，**绝不覆盖已有行的值**——设置项是用户在设置页直接改的，
// 每次启动无条件刷默认值等于静默丢弃用户的调整。默认值要变更时，走
// setting_schema 的 Default + 一次性存量刷新，而不是靠 seed 兜。
//
// 历史上有四条按「存量行」写的无条件 UPDATE（priority=0/NULL 回填、
// auto_associate NULL 回填、旧设置键迁移、request_total_timeout 刷默认值），
// 已随 AGENTS.md §0「不背历史包袱」一并删除：它们服务的存量行当前代码不再产生，
// 而其中两条会覆盖当前代码与用户正在用的合法值（详见
// seed_preserves_user_data_test.go 的两条回归）。
func seed(ctx context.Context) {
	initDefaultSettings(ctx)
}

// initDefaultSettings 初始化默认设置，所有默认值来自 SettingSchemas。
func initDefaultSettings(ctx context.Context) {
	for key, schema := range SettingSchemas() {
		value, err := SettingValueToString(schema.Default, schema.Type)
		if err != nil {
			panic(fmt.Sprintf("failed to convert default value for %s: %v", key, err))
		}

		count, err := gorm.G[Setting](DB).Where("key = ?", key).Count(ctx, "id")
		if err != nil {
			panic(err)
		}
		if count == 0 {
			if err := gorm.G[Setting](DB).Create(ctx, &Setting{Key: key, Value: value}); err != nil {
				panic(err)
			}
		}
	}
}
