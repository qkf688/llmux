package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"gorm.io/gorm"
)

// ModelSyncService 模型同步服务
type ModelSyncService struct {
	db *gorm.DB
}

// NewModelSyncService 创建模型同步服务实例
func NewModelSyncService(db *gorm.DB) *ModelSyncService {
	return &ModelSyncService{db: db}
}

// SyncProviderModels 同步单个提供商的上游模型
func (s *ModelSyncService) SyncProviderModels(ctx context.Context, providerID uint) (*models.ModelSyncLog, error) {
	// 获取提供商信息
	provider, err := gorm.G[models.Provider](s.db).Where("id = ?", providerID).First(ctx)
	if err != nil {
		slog.Error("failed to get provider", "provider_id", providerID, "error", err)
		return nil, err
	}

	// 检查是否支持模型端点（nil 视为 true）
	if provider.ModelEndpoint != nil && !*provider.ModelEndpoint {
		return nil, nil // 不支持模型端点，直接返回
	}

	// 获取当前上游模型
	currentModels := extractUpstreamModels(provider.Config)

	// 从上游获取模型列表
	config := provider.Config
	if cleanedConfig, err := dropCustomModels(config); err == nil {
		config = cleanedConfig
	}

	chatModel, err := providers.New(provider.Type, config, provider.Proxy)
	if err != nil {
		slog.Error("failed to create provider client", "provider_id", providerID, "error", err)
		return nil, err
	}

	upstreamModels, err := chatModel.Models(ctx)
	if err != nil {
		slog.Error("failed to fetch upstream models", "provider_id", providerID, "error", err)
		return nil, err
	}

	// 比较差异
	upstreamModelSet := make(map[string]bool)
	for _, model := range upstreamModels {
		upstreamModelSet[model.ID] = true
	}

	currentModelSet := make(map[string]bool)
	for _, modelID := range currentModels {
		currentModelSet[modelID] = true
	}

	// 计算新增和删除
	var addedModels []string
	var removedModels []string

	for _, model := range upstreamModels {
		if !currentModelSet[model.ID] {
			addedModels = append(addedModels, model.ID)
		}
	}

	for _, modelID := range currentModels {
		if !upstreamModelSet[modelID] {
			removedModels = append(removedModels, modelID)
		}
	}

	// 如果没有变化，不记录日志
	if len(addedModels) == 0 && len(removedModels) == 0 {
		return nil, nil
	}

	// 更新配置
	var updatedModels []string
	for _, model := range upstreamModels {
		updatedModels = append(updatedModels, model.ID)
	}

	newConfig := buildConfigWithAllModels(provider.Config, updatedModels)
	if _, err := gorm.G[models.Provider](s.db).Where("id = ?", providerID).Update(ctx, "config", newConfig); err != nil {
		return nil, err
	}

	// 创建同步日志
	syncLog := &models.ModelSyncLog{
		ProviderID:    providerID,
		ProviderName:  provider.Name,
		AddedCount:    len(addedModels),
		RemovedCount:  len(removedModels),
		AddedModels:   addedModels,
		RemovedModels: removedModels,
		SyncedAt:      time.Now(),
	}

	if err := gorm.G[models.ModelSyncLog](s.db).Create(ctx, syncLog); err != nil {
		slog.Error("failed to create sync log", "error", err)
		return nil, err
	}

	// 触发自动关联和清理
	s.triggerAutoActions(ctx, len(addedModels) > 0, len(removedModels) > 0)

	// 清理过期日志
	s.cleanOldLogs(ctx)

	return syncLog, nil
}

// SyncAllProviders 同步所有启用模型端点的提供商
func (s *ModelSyncService) SyncAllProviders(ctx context.Context) ([]*models.ModelSyncLog, error) {
	// 获取所有启用模型端点的提供商
	var providers []models.Provider
	if err := s.db.WithContext(ctx).
		Where("model_endpoint IS NULL OR model_endpoint = ?", true).
		Find(&providers).Error; err != nil {
		return nil, err
	}

	var logs []*models.ModelSyncLog
	for _, provider := range providers {
		log, err := s.SyncProviderModels(ctx, provider.ID)
		if err != nil {
			slog.Error("failed to sync provider", "provider_id", provider.ID, "error", err)
			continue
		}
		if log != nil {
			logs = append(logs, log)
		}
	}

	return logs, nil
}

// StartAutoSync 启动自动同步定时任务
func (s *ModelSyncService) StartAutoSync(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // 先1小时检查一次设置
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkAndSync(ctx)
			}
		}
	}()
}

func (s *ModelSyncService) checkAndSync(ctx context.Context) {
	// 读取设置
	enabled, err := s.getSettingBool(ctx, models.SettingKeyModelSyncEnabled)
	if err != nil || !enabled {
		return
	}

	interval, err := s.getSettingInt(ctx, models.SettingKeyModelSyncInterval)
	if err != nil {
		interval = 12 // 默认12小时
	}

	// 检查上次同步时间
	var lastLog models.ModelSyncLog
	if err := s.db.WithContext(ctx).Order("synced_at DESC").First(&lastLog).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			slog.Error("failed to get last sync log", "error", err)
			return
		}
		// 没有日志记录，执行同步
		s.SyncAllProviders(ctx)
		return
	}

	// 检查是否到达同步间隔
	if time.Since(lastLog.SyncedAt) >= time.Duration(interval)*time.Hour {
		s.SyncAllProviders(ctx)
	}
}

func (s *ModelSyncService) cleanOldLogs(ctx context.Context) {
	// 获取保留设置
	retentionCount, err := s.getSettingInt(ctx, models.SettingKeyModelSyncLogRetentionCount)
	if err != nil {
		retentionCount = 100
	}

	retentionDays, err := s.getSettingInt(ctx, models.SettingKeyModelSyncLogRetentionDays)
	if err != nil {
		retentionDays = 7
	}

	// 按条数清理
	if retentionCount > 0 {
		var count int64
		s.db.WithContext(ctx).Model(&models.ModelSyncLog{}).Count(&count)
		if count > int64(retentionCount) {
			// 删除最旧的记录
			toDelete := count - int64(retentionCount)
			if _, err := gorm.G[models.ModelSyncLog](s.db).
				Order("synced_at ASC").
				Limit(int(toDelete)).
				Delete(ctx); err != nil {
				slog.Error("failed to clean old logs by count", "error", err)
			}
		}
	}

	// 按天数清理
	if retentionDays > 0 {
		cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
		if _, err := gorm.G[models.ModelSyncLog](s.db).
			Where("synced_at < ?", cutoffTime).
			Delete(ctx); err != nil {
			slog.Error("failed to clean old logs by days", "error", err)
		}
	}
}

func (s *ModelSyncService) getSettingBool(ctx context.Context, key string) (bool, error) {
	setting, err := gorm.G[models.Setting](s.db).Where("key = ?", key).First(ctx)
	if err != nil {
		return false, err
	}
	return setting.Value == "true", nil
}

func (s *ModelSyncService) getSettingInt(ctx context.Context, key string) (int, error) {
	setting, err := gorm.G[models.Setting](s.db).Where("key = ?", key).First(ctx)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(setting.Value)
}

func dropCustomModels(config string) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", err
	}

	delete(parsed, "custom_models")
	delete(parsed, "upstream_models")

	updated, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(updated), nil
}

func extractAllModels(config string) []string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return []string{}
	}

	var models []string
	if upstream, ok := parsed["upstream_models"].([]interface{}); ok {
		for _, m := range upstream {
			if str, ok := m.(string); ok {
				models = append(models, str)
			}
		}
	}
	if custom, ok := parsed["custom_models"].([]interface{}); ok {
		for _, m := range custom {
			if str, ok := m.(string); ok {
				models = append(models, str)
			}
		}
	}
	return models
}

func extractUpstreamModels(config string) []string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return []string{}
	}

	upstream, ok := parsed["upstream_models"].([]interface{})
	if !ok {
		return []string{}
	}

	var models []string
	for _, m := range upstream {
		if str, ok := m.(string); ok {
			models = append(models, str)
		}
	}
	return models
}

func buildConfigWithAllModels(config string, modelList []string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return config
	}

	parsed["upstream_models"] = modelList

	updated, err := json.Marshal(parsed)
	if err != nil {
		return config
	}
	return string(updated)
}

// GetProviderModels 获取提供商的所有模型列表
func GetProviderModels(ctx context.Context, provider models.Provider) ([]string, error) {
	return extractAllModels(provider.Config), nil
}

// triggerAutoActions 触发自动关联和清理操作
func (s *ModelSyncService) triggerAutoActions(ctx context.Context, hasAdded bool, hasRemoved bool) {
	// 检查是否启用自动关联
	if hasAdded {
		autoAssociate, _ := s.getSettingBool(ctx, models.SettingKeyAutoAssociateOnAdd)
		if autoAssociate {
			// 使用不随请求取消的 Context，避免异步任务被提前中止
			go s.autoAssociateModels(context.WithoutCancel(ctx))
		}
	}

	// 检查是否启用自动清理
	if hasRemoved {
		autoClean, _ := s.getSettingBool(ctx, models.SettingKeyAutoCleanOnDelete)
		if autoClean {
			// 使用不随请求取消的 Context，避免异步任务被提前中止
			go s.cleanInvalidAssociations(context.WithoutCancel(ctx))
		}
	}
}

// autoAssociateModels 自动关联模型
func (s *ModelSyncService) autoAssociateModels(ctx context.Context) {
	allModels, err := gorm.G[models.Model](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get models for auto-associate", "error", err)
		return
	}

	allProviders, err := gorm.G[models.Provider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get providers for auto-associate", "error", err)
		return
	}

	existingAssociations, err := gorm.G[models.ModelWithProvider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get existing associations", "error", err)
		return
	}

	manualTemplateItems, err := gorm.G[models.ModelTemplateItem](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get template items for auto-associate", "error", err)
		return
	}

	existingMap := make(map[string]bool)
	for _, assoc := range existingAssociations {
		key := fmt.Sprintf("%d_%d_%s", assoc.ModelID, assoc.ProviderID, assoc.ProviderModel)
		existingMap[key] = true
	}

	templateIndex := BuildTemplateIndexFromData(allModels, existingAssociations, manualTemplateItems)

	defaultPriority := 10
	if setting, err := gorm.G[models.Setting](s.db).Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).First(ctx); err == nil {
		if val, err := strconv.Atoi(setting.Value); err == nil {
			defaultPriority = val
		}
	}

	addedCount := 0
	for _, provider := range allProviders {
		providerModels := extractAllModels(provider.Config)
		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
				key := fmt.Sprintf("%d_%d_%s", modelID, provider.ID, providerModel)
				if existingMap[key] {
					continue
				}
				trueVal := true
				falseVal := false
				newAssoc := models.ModelWithProvider{
					ModelID:          modelID,
					ProviderModel:    providerModel,
					ProviderID:       provider.ID,
					ToolCall:         &trueVal,
					StructuredOutput: &falseVal,
					Image:            &falseVal,
					WithHeader:       &falseVal,
					Status:           &trueVal,
					CustomerHeaders:  map[string]string{},
					Weight:           5,
					Priority:         defaultPriority,
				}
				if err := gorm.G[models.ModelWithProvider](s.db).Create(ctx, &newAssoc); err == nil {
					existingMap[key] = true
					addedCount++
				}
			}
		}
	}

	if addedCount > 0 {
		slog.Info("auto-associated models", "count", addedCount)
	}
}

// cleanInvalidAssociations 清理无效关联
func (s *ModelSyncService) cleanInvalidAssociations(ctx context.Context) {
	allAssociations, err := gorm.G[models.ModelWithProvider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get associations for auto-clean", "error", err)
		return
	}

	allProviders, err := gorm.G[models.Provider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get providers for auto-clean", "error", err)
		return
	}

	providerMap := make(map[uint]*models.Provider)
	for i := range allProviders {
		providerMap[allProviders[i].ID] = &allProviders[i]
	}

	removedCount := 0
	for _, assoc := range allAssociations {
		shouldDelete := false
		provider, providerExists := providerMap[assoc.ProviderID]
		if !providerExists {
			shouldDelete = true
		} else {
			providerModels := extractAllModels(provider.Config)
			modelExists := false
			for _, pm := range providerModels {
				if pm == assoc.ProviderModel {
					modelExists = true
					break
				}
			}
			if !modelExists {
				shouldDelete = true
			}
		}

		if shouldDelete {
			if _, err := gorm.G[models.ModelWithProvider](s.db).Where("id = ?", assoc.ID).Delete(ctx); err == nil {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		slog.Info("auto-cleaned invalid associations", "count", removedCount)
	}
}
