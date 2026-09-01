package modelsync

import (
	"context"
	"time"

	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/channel"
	"gorm.io/gorm"
)

// ActionHooks 用于注入模型同步完成后的扩展动作。
type ActionHooks struct {
	AutoAssociate            func(ctx context.Context)
	CleanInvalidAssociations func(ctx context.Context)
}

// Service 模型同步服务。
type Service struct {
	db        *gorm.DB
	hooks     ActionHooks
	repos     *repository.Repositories
	assembler *channel.Assembler
}

// NewService 创建模型同步服务实例。
func NewService(db *gorm.DB, hooks ActionHooks) *Service {
	repos := repository.New(db)
	return &Service{
		db:        db,
		hooks:     hooks,
		repos:     repos,
		assembler: channel.NewAssembler(repos),
	}
}

// AddedModel 新增模型信息。
type AddedModel struct {
	ModelName    string    `json:"model_name"`
	ProviderName string    `json:"provider_name"`
	AddedAt      time.Time `json:"added_at"`
}

// RecentAddedModelsResponse 最近新增模型响应。
type RecentAddedModelsResponse struct {
	Data       []AddedModel `json:"data"`
	SyncTime   *time.Time   `json:"sync_time,omitempty"`
	TotalCount int          `json:"total_count"`
}
