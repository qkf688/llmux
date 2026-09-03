package models

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gorm.io/gorm"
)

// migrate AutoMigrate + 一次性兼容/清理。
func migrate(ctx context.Context) {
	if err := DB.AutoMigrate(
		&Provider{},
		&Pool{},
		&Credential{},
		&Endpoint{},
		&KeyGroup{},
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
		&User{},
	); err != nil {
		panic(err)
	}
	migrateLegacyProviders(ctx, DB, credentialcrypto.Default())
	purgeLegacyUpstreamModels(ctx, DB)
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

// purgeLegacyUpstreamModels 剥离存量 Provider.Config 中的遗留死键 upstream_models
// （S5 起后端零读写、#18 起前端零读写，键值无消费者，清掉零丢失）。
// 幂等：键存在才改写，无键行零 Update；非 JSON 行按已损坏跳过，不阻断启动。
func purgeLegacyUpstreamModels(ctx context.Context, db *gorm.DB) {
	var providers []Provider
	if err := db.WithContext(ctx).Find(&providers).Error; err != nil {
		panic(err)
	}
	for i := range providers {
		config := providers[i].Config
		// 空串是合法状态（parseConfigMap 同样容忍），必然不含死键，静默跳过。
		if config == "" {
			continue
		}
		if !gjson.Valid(config) {
			slog.Warn("migrate: provider config is not valid JSON, skip upstream_models purge", "provider_id", providers[i].ID)
			continue
		}
		if !gjson.Get(config, "upstream_models").Exists() {
			continue
		}
		cleaned, err := sjson.Delete(config, "upstream_models")
		if err != nil {
			panic(fmt.Errorf("delete upstream_models from config for provider %d: %w", providers[i].ID, err))
		}
		if err := db.WithContext(ctx).Model(&providers[i]).Update("config", cleaned).Error; err != nil {
			panic(fmt.Errorf("update config for provider %d: %w", providers[i].ID, err))
		}
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
