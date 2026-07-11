package models

import (
	"context"

	"gorm.io/gorm"
)

// migrate AutoMigrate + 一次性兼容/清理。
func migrate(ctx context.Context) {
	if err := DB.AutoMigrate(
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