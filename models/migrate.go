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
	purgeSoftDeleted(ctx, &VirtualModelMapping{}, &ModelTemplateItem{}, &VirtualModel{}, &Setting{})
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

// purgeSoftDeleted 物理清除历史软删残留行。
// 这些表的唯一索引不含 deleted_at，残留行不可见却会挡住同 key 重建；
// 其级联清理已改为 Unscoped 硬删，此处只负责抹掉改动前遗留的数据。
func purgeSoftDeleted(ctx context.Context, dests ...any) {
	for _, dest := range dests {
		if err := DB.WithContext(ctx).
			Unscoped().
			Where("deleted_at IS NOT NULL").
			Delete(dest).Error; err != nil {
			panic(err)
		}
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