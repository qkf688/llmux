package models

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// seed 默认设置、字段回填、旧键迁移（含一次性数据修复）。
func seed(ctx context.Context) {
	initDefaultSettings(ctx)
	initPriorityField(ctx)
	initAutoAssociateField(ctx)
	migrateSettingKeys(ctx)
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

// initPriorityField 初始化优先级字段，为现有记录设置默认优先级
func initPriorityField(ctx context.Context) {
	// 为 priority 为 0 的记录设置默认优先级 10
	if _, err := gorm.G[ModelWithProvider](DB).Where("priority = 0 OR priority IS NULL").Update(ctx, "priority", 10); err != nil {
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