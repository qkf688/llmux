package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// seed 默认设置、字段回填、旧键迁移（含一次性数据修复）。
func seed(ctx context.Context) {
	initDefaultSettings(ctx)
	initPriorityField(ctx)
	initAutoAssociateField(ctx)
	migrateSettingKeys(ctx)
	migrateRequestTotalTimeoutToCurrentDefault(ctx)
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

// initPriorityField 初始化优先级字段，为现有记录设置默认优先级。
// 默认值来自 setting schema（与 handler/settings.ResetModelPriorities 同源），
// 保证种子回填与重置值一致，避免硬编码漂移。
func initPriorityField(ctx context.Context) {
	defaultPriority := GetSettingInt(ctx, SettingKeyAutoPriorityDecayDefault, 100, 0)
	if _, err := gorm.G[ModelWithProvider](DB).Where("priority = 0 OR priority IS NULL").Update(ctx, "priority", defaultPriority); err != nil {
		panic(err)
	}
}

// initAutoAssociateField 初始化自动关联字段，为现有模型设置默认值
func initAutoAssociateField(ctx context.Context) {
	// 为 auto_associate 为 NULL 的记录设置默认值 true
	if _, err := gorm.G[Model](DB).Where("auto_associate IS NULL").Update(ctx, "auto_associate", true); err != nil {
		panic(err)
	}
}

// migrateSettingKeys 迁移旧设置键到新设置键
func migrateSettingKeys(ctx context.Context) {
	// 迁移 auto_save_template_on_delete 到 auto_save_template_on_associate
	oldKey := "auto_save_template_on_delete"
	newKey := SettingKeyAutoSaveTemplateOnAssociate

	// 检查旧键是否存在
	oldSetting, err := gorm.G[Setting](DB).Where("key = ?", oldKey).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		panic(err)
	}

	// 检查新键是否已存在
	count, err := gorm.G[Setting](DB).Where("key = ?", newKey).Count(ctx, "id")
	if err != nil {
		panic(err)
	}

	// 如果新键不存在，则创建新键并复制旧键的值
	if count == 0 {
		newSetting := Setting{
			Key:   newKey,
			Value: oldSetting.Value,
		}
		if err := gorm.G[Setting](DB).Create(ctx, &newSetting); err != nil {
			panic(err)
		}
	}

	// 删除旧键
	if _, err := gorm.G[Setting](DB).Where("key = ?", oldKey).Delete(ctx); err != nil {
		panic(err)
	}
}

// migrateRequestTotalTimeoutToCurrentDefault 启动时一次性迁移：
// 把 Setting 表里 key=request_total_timeout 的所有行的 value 列无条件刷为当前默认值。
//
// 与 migrateSettingKeys 的差异：migrateSettingKeys 按"换键名"做迁移（auto_save_template_on_delete →
// auto_save_template_on_associate）；本函数按"统一当前默认值"做迁移——默认值的语义随 schema 调整而
// 调整（Stage 33 起从 90 改为 600），存量行 value 列必须跟上，否则网关启动后仍按旧默认行为运行。
//
// 不写条件改写、不保护"人工调过的值"（AGENTS.md §0.1：开发阶段不背历史包袱；当前部署仅开发者本人
// 使用，无真实存量约束）。若有"人工调过的值"也按当前默认值覆盖——人工调整应基于当前默认值的语义，
// 旧 schema 上的调整在新 schema 上已无意义。
//
// 失败处置：slog.Warn + continue，不 panic。数据迁移失败不应让 startup 挂掉，按 §0.1 推论旧
// 数据不应绑架 startup。
func migrateRequestTotalTimeoutToCurrentDefault(ctx context.Context) {
	schema, ok := SettingSchemas()[SettingKeyRequestTotalTimeout]
	if !ok {
		// schema 未注册——理论上 seed 流程保证就位；这条 panic 是防御性的，定位用
		panic(fmt.Sprintf("request_total_timeout schema not registered"))
	}
	defaultValue, err := SettingValueToString(schema.Default, schema.Type)
	if err != nil {
		panic(fmt.Sprintf("failed to convert default value for %s: %v", SettingKeyRequestTotalTimeout, err))
	}
	if _, err := gorm.G[Setting](DB).Where("key = ?", SettingKeyRequestTotalTimeout).Update(ctx, "value", defaultValue); err != nil {
		slog.Warn("failed to migrate request_total_timeout to current default; continuing with existing value",
			"key", SettingKeyRequestTotalTimeout, "default", defaultValue, "error", err)
	}
}
