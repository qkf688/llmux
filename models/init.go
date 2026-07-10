package models

import (
	"context"
	"errors"
	"fmt"
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
		&ModelTemplateItem{},
		&VirtualModel{},
		&VirtualModelMapping{},
		&ChatLog{},
		&ChatIO{},
		&StatsTotal{},
		&StatsDaily{},
		&StatsHourly{},
		&StatsModelTotal{},
		&StatsRealModelTotal{},
		&StatsProviderTotal{},
		&Setting{},
		&HealthCheckLog{},
		&ModelSyncLog{},
	); err != nil {
		panic(err)
	}
	cleanupVirtualModelMappingSoftDeletes(ctx)
	cleanupSoftDeletedModels(ctx)
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
	// 初始化优先级字段
	initPriorityField(ctx)
	// 初始化自动关联字段
	initAutoAssociateField(ctx)
	// 迁移旧设置键
	migrateSettingKeys(ctx)
}

func cleanupVirtualModelMappingSoftDeletes(ctx context.Context) {
	if err := DB.WithContext(ctx).
		Unscoped().
		Where("deleted_at IS NOT NULL").
		Delete(&VirtualModelMapping{}).Error; err != nil {
		panic(err)
	}
}

func cleanupSoftDeletedModels(ctx context.Context) {
	if !DB.Migrator().HasColumn(&Model{}, "deleted_at") {
		return
	}

	var softDeletedModels []Model
	if err := DB.WithContext(ctx).
		Unscoped().
		Where("deleted_at IS NOT NULL").
		Find(&softDeletedModels).Error; err != nil {
		panic(err)
	}

	for _, m := range softDeletedModels {
		if err := DB.WithContext(ctx).
			Unscoped().
			Where("model_id = ?", m.ID).
			Delete(&ModelWithProvider{}).Error; err != nil {
			panic(err)
		}
		if err := DB.WithContext(ctx).
			Unscoped().
			Where("model_id = ?", m.ID).
			Delete(&ModelTemplateItem{}).Error; err != nil {
			panic(err)
		}
		if err := DB.WithContext(ctx).
			Unscoped().
			Where("real_model_id = ?", m.ID).
			Delete(&VirtualModelMapping{}).Error; err != nil {
			panic(err)
		}
		if err := DB.WithContext(ctx).
			Unscoped().
			Where("id = ?", m.ID).
			Delete(&Model{}).Error; err != nil {
			panic(err)
		}
	}

	if err := DB.Migrator().DropColumn(&Model{}, "deleted_at"); err != nil {
		panic(err)
	}
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
