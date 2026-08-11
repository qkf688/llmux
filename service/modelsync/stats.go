package modelsync

import (
	"context"
	"fmt"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// SyncStatsResult 模型同步统计结果，对应 handler 的响应 DTO。
type SyncStatsResult struct {
	LastSyncAt           *time.Time
	NextSyncAt           *time.Time
	SyncEnabled          bool
	SyncInterval         int
	TotalProviders       int64
	ProvidersWithUpdates int
	ProvidersUnchanged   int
	ProvidersWithErrors  int
	ProvidersNeverSynced int
}

// GetSyncStats 汇总同步统计：供应商数 + 各状态计数 + 上次/下次同步时间。
// 设置读取复用 service 内部 getSettingBool/getSettingInt（仍为存量直连，不在本方法范围内迁移）。
func (s *Service) GetSyncStats(ctx context.Context, repos *repository.Repositories) (*SyncStatsResult, error) {
	// 1. 统计启用模型端点的供应商
	totalProviders, err := repos.Provider.CountEnabledModelEndpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("count enabled providers: %w", err)
	}

	// 2. 获取启用的 provider IDs（喂给 log repo 聚合）
	providerIDs, err := repos.Provider.EnabledModelEndpointIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("get enabled provider IDs: %w", err)
	}

	// 3. 从 ModelSyncLogRepo 聚合 per-provider 最新状态
	stats, err := repos.ModelSyncLog.AggregateProviderStats(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("aggregate provider stats: %w", err)
	}

	// 4. 读同步设置（仍走 service 内部 gorm.G 直连——存量豁免，不在 #1 范围）
	syncEnabled, _ := s.getSettingBool(ctx, models.SettingKeyModelSyncEnabled)
	syncInterval, err := s.getSettingInt(ctx, models.SettingKeyModelSyncInterval)
	if err != nil || syncInterval <= 0 {
		syncInterval = 12
	}

	// 5. 计算下次同步时间
	var nextSyncAt *time.Time
	if syncEnabled && stats.LastSyncAt != nil {
		next := stats.LastSyncAt.Add(time.Duration(syncInterval) * time.Hour)
		nextSyncAt = &next
	}

	// 6. 从未同步的供应商
	neverSynced := int(totalProviders) - stats.ProvidersSynced
	if neverSynced < 0 {
		neverSynced = 0
	}

	return &SyncStatsResult{
		LastSyncAt:           stats.LastSyncAt,
		NextSyncAt:           nextSyncAt,
		SyncEnabled:          syncEnabled,
		SyncInterval:         syncInterval,
		TotalProviders:       totalProviders,
		ProvidersWithUpdates: stats.ProvidersWithUpdates,
		ProvidersUnchanged:   stats.ProvidersUnchanged,
		ProvidersWithErrors:  stats.ProvidersWithErrors,
		ProvidersNeverSynced: neverSynced,
	}, nil
}
