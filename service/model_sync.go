package service

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/autoassoc"
	"github.com/atopos31/llmio/service/modelsync"
	"gorm.io/gorm"
)

// ModelSyncService 模型同步服务（兼容层）。
type ModelSyncService struct {
	db   *gorm.DB
	core *modelsync.Service
}

// AddedModel 新增模型信息。
type AddedModel = modelsync.AddedModel

// RecentAddedModelsResponse 最近新增模型响应。
type RecentAddedModelsResponse = modelsync.RecentAddedModelsResponse

// NewModelSyncService 创建模型同步服务实例。
// auto 为 nil 时使用 GetAutoAssocService() 默认单例。
// 同步后 ActionHooks 委托统一 autoassoc 服务，避免与 handler 双轨实现。
func NewModelSyncService(db *gorm.DB, auto *autoassoc.Service) *ModelSyncService {
	if auto == nil {
		auto = GetAutoAssocService()
	}
	svc := &ModelSyncService{db: db}
	svc.core = modelsync.NewService(db, modelsync.ActionHooks{
		AutoAssociate: func(ctx context.Context) {
			// 开关已在 modelsync.triggerAutoActions 检查；此处直接执行业务。
			added, err := auto.Associate(ctx)
			if err != nil {
				slog.Error("auto-associate failed", "error", err, "added", added)
				return
			}
			if added > 0 {
				slog.Info("auto-associated models", "count", added)
			}
		},
		CleanInvalidAssociations: func(ctx context.Context) {
			removed, err := auto.CleanInvalid(ctx)
			if err != nil {
				slog.Error("auto-clean failed", "error", err, "removed", removed)
				return
			}
			if removed > 0 {
				slog.Info("auto-cleaned invalid associations", "count", removed)
			}
		},
	})
	return svc
}

// SyncProviderModels 同步单个提供商的上游模型。
func (s *ModelSyncService) SyncProviderModels(ctx context.Context, providerID uint) (*models.ModelSyncLog, error) {
	return s.core.SyncProviderModels(ctx, providerID)
}

// SyncAllProviders 同步所有启用模型端点的提供商。
func (s *ModelSyncService) SyncAllProviders(ctx context.Context) ([]*models.ModelSyncLog, error) {
	return s.core.SyncAllProviders(ctx)
}

// GetRecentAddedModels 获取最近一次同步批次新增的模型。
func (s *ModelSyncService) GetRecentAddedModels(ctx context.Context) (*RecentAddedModelsResponse, error) {
	return s.core.GetRecentAddedModels(ctx)
}

// StartAutoSync 启动自动同步定时任务。
func (s *ModelSyncService) StartAutoSync(ctx context.Context) {
	s.core.StartAutoSync(ctx)
}

// GetProviderModels 获取提供商的所有模型列表。
func GetProviderModels(ctx context.Context, provider models.Provider) ([]string, error) {
	return modelsync.GetProviderModels(ctx, provider)
}
