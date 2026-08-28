// Package repository 封装对持久层（GORM/SQLite）的访问，为 service/handler 提供与数据访问框架解耦的接口。
package repository

import (
	"context"
	"sync"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// Repositories 聚合所有 Repository 实例，便于在应用启动时统一初始化与注入。
type Repositories struct {
	db *gorm.DB

	Provider            ProviderRepo
	Model               ModelRepo
	ModelWithProvider   ModelWithProviderRepo
	ModelTemplateItem   ModelTemplateItemRepo
	VirtualModel        VirtualModelRepo
	VirtualModelMapping VirtualModelMappingRepo
	Setting             SettingRepo
	ChatLog             ChatLogRepo
	ChatIO              ChatIORepo
	HealthCheckLog      HealthCheckLogRepo
	ModelSyncLog        ModelSyncLogRepo
	Stats               StatsRepo
	User                UserRepo
	Pool                PoolRepo
	Credential          CredentialRepo
	Endpoint            EndpointRepo
	KeyGroup            KeyGroupRepo
}

// New 根据 *gorm.DB 创建所有 Repository 实例。
// 传入的 db 可以是根连接，也可以是事务内的 *gorm.DB。
func New(db *gorm.DB) *Repositories {
	return &Repositories{
		db:                  db,
		Provider:            NewProviderRepo(db),
		Model:               NewModelRepo(db),
		ModelWithProvider:   NewModelWithProviderRepo(db),
		ModelTemplateItem:   NewModelTemplateItemRepo(db),
		VirtualModel:        NewVirtualModelRepo(db),
		VirtualModelMapping: NewVirtualModelMappingRepo(db),
		Setting:             NewSettingRepo(db),
		ChatLog:             NewChatLogRepo(db),
		ChatIO:              NewChatIORepo(db),
		HealthCheckLog:      NewHealthCheckLogRepo(db),
		ModelSyncLog:        NewModelSyncLogRepo(db),
		Stats:               NewStatsRepo(db),
		User:                NewUserRepo(db),
		Pool:                NewPoolRepo(db),
		Credential:          NewCredentialRepo(db),
		Endpoint:            NewEndpointRepo(db),
		KeyGroup:            NewKeyGroupRepo(db),
	}
}

// RunInTx 在同一事务中执行 fn；fn 收到的 Repositories 绑定事务 db。
// 失败回滚，成功提交。对齐 GORM Transaction 语义。
func (r *Repositories) RunInTx(ctx context.Context, fn func(txRepos *Repositories) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(New(tx))
	})
}

var (
	defaultMu    sync.RWMutex
	defaultRepos *Repositories
)

// SetDefault 设置包级默认 Repositories（通常在 models.Init 之后调用一次）。
func SetDefault(r *Repositories) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultRepos = r
}

// Default 返回包级默认 Repositories。
// 未显式 SetDefault 时，在 models.DB 已初始化的前提下懒创建。
func Default() *Repositories {
	defaultMu.RLock()
	r := defaultRepos
	defaultMu.RUnlock()
	if r != nil {
		return r
	}

	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultRepos == nil {
		if models.DB == nil {
			panic("repository: models.DB is nil; call models.Init before repository.Default")
		}
		defaultRepos = New(models.DB)
	}
	return defaultRepos
}
