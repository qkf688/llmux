package modelsync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"gorm.io/gorm"
)

// SyncProviderModels 按分组粒度同步上游模型：每组用活跃凭据打 Models()，
// 全量写入该组 KeyGroup.Models；请求数 ≈ 分组数。日志仍按供应商聚合。
func (s *Service) SyncProviderModels(ctx context.Context, providerID uint) (*models.ModelSyncLog, error) {
	provider, err := gorm.G[models.Provider](s.db).Where("id = ?", providerID).First(ctx)
	if err != nil {
		slog.Error("failed to get provider", "provider_id", providerID, "error", err)
		return nil, err
	}

	beforeModels, err := collectGroupWhitelistUnion(ctx, s.repos, providerID)
	if err != nil {
		return nil, fmt.Errorf("list group models before sync: %w", err)
	}

	snapshot, err := s.assembler.Assemble(ctx, provider)
	if err != nil {
		slog.Error("failed to assemble channel snapshot", "provider_id", providerID, "error", err)
		return s.createAndSaveErrorLog(ctx, provider, err.Error()), nil
	}

	if len(snapshot.Groups) == 0 {
		return s.createAndSaveErrorLog(ctx, provider, "no key groups to sync"), nil
	}

	now := time.Now()
	var (
		syncedCount int
		skipReasons []string
		fetchErrs   []string
	)

	for _, group := range snapshot.Groups {
		outcome := s.syncKeyGroup(ctx, provider, snapshot, group, now)
		if outcome.Skipped {
			skipReasons = append(skipReasons, fmt.Sprintf("group %q: %s", outcome.GroupName, outcome.SkipReason))
			continue
		}
		if outcome.Err != nil {
			slog.Error("group model sync failed",
				"provider_id", providerID, "group_id", outcome.GroupID, "error", outcome.Err)
			fetchErrs = append(fetchErrs, outcome.Err.Error())
			continue
		}
		syncedCount++
	}

	if syncedCount == 0 {
		msg := "all key groups skipped or failed"
		if len(fetchErrs) > 0 {
			msg = strings.Join(fetchErrs, "; ")
		} else if len(skipReasons) > 0 {
			msg = strings.Join(skipReasons, "; ")
		}
		return s.createAndSaveErrorLog(ctx, provider, msg), nil
	}

	afterModels, err := collectGroupWhitelistUnion(ctx, s.repos, providerID)
	if err != nil {
		return nil, fmt.Errorf("list group models after sync: %w", err)
	}

	addedModels, removedModels := diffStringSets(beforeModels, afterModels)
	hasChanges := len(addedModels) > 0 || len(removedModels) > 0
	status := "success"
	if !hasChanges {
		status = "unchanged"
	}

	errMsg := ""
	if len(fetchErrs) > 0 || len(skipReasons) > 0 {
		parts := append(append([]string{}, fetchErrs...), skipReasons...)
		errMsg = strings.Join(parts, "; ")
	}

	syncLog := &models.ModelSyncLog{
		ProviderID:    providerID,
		ProviderName:  provider.Name,
		Status:        status,
		Error:         errMsg,
		AddedCount:    len(addedModels),
		RemovedCount:  len(removedModels),
		AddedModels:   addedModels,
		RemovedModels: removedModels,
		SyncedAt:      time.Now(),
	}

	if err := gorm.G[models.ModelSyncLog](s.db).Create(ctx, syncLog); err != nil {
		slog.Error("failed to create sync log", "provider_id", providerID, "error", err)
		return nil, err
	}

	if hasChanges {
		s.triggerAutoActions(ctx, len(addedModels) > 0, len(removedModels) > 0)
	}

	s.cleanOldLogs(ctx)

	return syncLog, nil
}

func (s *Service) createAndSaveErrorLog(ctx context.Context, provider models.Provider, message string) *models.ModelSyncLog {
	syncLog := &models.ModelSyncLog{
		ProviderID:    provider.ID,
		ProviderName:  provider.Name,
		Status:        "error",
		Error:         message,
		AddedCount:    0,
		RemovedCount:  0,
		AddedModels:   []string{},
		RemovedModels: []string{},
		SyncedAt:      time.Now(),
	}
	if err := gorm.G[models.ModelSyncLog](s.db).Create(ctx, syncLog); err != nil {
		slog.Error("failed to create sync log", "provider_id", provider.ID, "error", err)
	}
	return syncLog
}

func collectUpstreamModelIDs(upstreamModels []providers.Model) []string {
	modelIDs := make([]string, 0, len(upstreamModels))
	for _, model := range upstreamModels {
		modelIDs = append(modelIDs, model.ID)
	}
	return modelIDs
}

func diffStringSets(before, after []string) (added, removed []string) {
	afterSet := make(map[string]struct{}, len(after))
	for _, id := range after {
		afterSet[id] = struct{}{}
	}
	beforeSet := make(map[string]struct{}, len(before))
	for _, id := range before {
		beforeSet[id] = struct{}{}
	}

	added = make([]string, 0)
	for _, id := range after {
		if _, ok := beforeSet[id]; !ok {
			added = append(added, id)
		}
	}
	removed = make([]string, 0)
	for _, id := range before {
		if _, ok := afterSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	return added, removed
}
