package autoassoc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/atopos31/llmio/service/modelsync"
)

// Service 自动关联与无效关联清理的统一入口。
// HTTP / provider CRUD / modelsync ActionHooks 均应调用本服务，避免双轨规则漂移。
type Service struct {
	repos      *repository.Repositories
	buildIndex BuildIndexFunc
}

// NewService 创建自动关联服务。
// repos 为 nil 时使用 repository.Default()；buildIndex 为 nil 时 panic（必须注入模板索引构建）。
func NewService(repos *repository.Repositories, buildIndex BuildIndexFunc) *Service {
	if buildIndex == nil {
		panic("autoassoc: buildIndex is required")
	}
	return &Service{
		repos:      repos,
		buildIndex: buildIndex,
	}
}

func (s *Service) repositories() *repository.Repositories {
	if s.repos != nil {
		return s.repos
	}
	return repository.Default()
}

func (s *Service) settingBool(ctx context.Context, key string, defaultValue bool) bool {
	val, err := s.repositories().Setting.GetValue(ctx, key)
	if err != nil || val == "" {
		return defaultValue
	}
	return val == "true"
}

func (s *Service) settingInt(ctx context.Context, key string, defaultValue, minValue int) int {
	val, err := s.repositories().Setting.GetValue(ctx, key)
	if err != nil || val == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < minValue {
		return defaultValue
	}
	return n
}

// PreviewAssociate 预览将要创建的缺失关联（尊重 Model.AutoAssociate、跳过黑名单与已存在 key）。
func (s *Service) PreviewAssociate(ctx context.Context) ([]Preview, error) {
	data, err := s.fetchAssociateData(ctx)
	if err != nil {
		return nil, err
	}

	previews := make([]Preview, 0)
	s.forEachMissingAssociation(ctx, data, nil, func(candidate associationCandidate, _ string, _ map[string]bool) {
		previews = append(previews, Preview{
			ModelID:       candidate.ModelID,
			ModelName:     candidate.ModelName,
			ProviderID:    candidate.ProviderID,
			ProviderName:  candidate.ProviderName,
			ProviderModel: candidate.ProviderModel,
		})
	})
	return previews, nil
}

// Associate 执行自动关联，返回成功创建条数。不检查全局开关（由调用方决定是否触发）。
// 若存在写库失败，返回已成功条数与 error（部分失败可观测）。
func (s *Service) Associate(ctx context.Context) (int, error) {
	data, err := s.fetchAssociateData(ctx)
	if err != nil {
		return 0, err
	}

	defaultPriority := s.settingInt(ctx, models.SettingKeyAutoPriorityDecayDefault, DefaultPriorityFallback, 0)
	addedCount := 0
	failedCount := 0
	repos := s.repositories()

	s.forEachMissingAssociation(
		ctx,
		data,
		func(provider models.Provider, err error) {
			slog.Warn("failed to get provider models", "provider", provider.Name, "error", err)
		},
		func(candidate associationCandidate, key string, existingMap map[string]bool) {
			newAssoc := NewDefaultAssociation(
				candidate.ModelID,
				candidate.ProviderID,
				candidate.ProviderModel,
				defaultPriority,
			)
			if err := repos.ModelWithProvider.Create(ctx, &newAssoc); err != nil {
				failedCount++
				slog.Warn("failed to create association",
					"model", candidate.ModelName,
					"provider", candidate.ProviderName,
					"error", err,
				)
				return
			}
			existingMap[key] = true
			addedCount++
		},
	)

	if failedCount > 0 {
		return addedCount, fmt.Errorf("auto-associate: %d created, %d failed", addedCount, failedCount)
	}
	return addedCount, nil
}

// TriggerAssociateIfEnabled 若全局开关开启则执行 Associate（供 provider CRUD / 旁路使用）。
func (s *Service) TriggerAssociateIfEnabled(ctx context.Context) {
	if !s.settingBool(ctx, models.SettingKeyAutoAssociateOnAdd, false) {
		slog.Info("auto-associate disabled")
		return
	}
	slog.Info("auto-associate triggered")

	addedCount, err := s.Associate(ctx)
	if err != nil {
		slog.Error("auto-associate failed", "error", err, "added", addedCount)
		return
	}
	if addedCount > 0 {
		slog.Info("auto-associated models", "count", addedCount)
	}
}

// PreviewClean 预览将要删除的无效关联。
func (s *Service) PreviewClean(ctx context.Context) ([]Preview, error) {
	data, err := s.fetchCleanData(ctx, true)
	if err != nil {
		return nil, err
	}
	modelByID := indexModelsByID(data.allModels)

	previews := make([]Preview, 0)
	s.forEachInvalidAssociation(ctx, data, nil, func(assoc models.ModelWithProvider, provider *models.Provider) {
		providerName := ""
		if provider != nil {
			providerName = provider.Name
		}
		modelName := ""
		if model, ok := modelByID[assoc.ModelID]; ok {
			modelName = model.Name
		}
		previews = append(previews, Preview{
			ModelID:       assoc.ModelID,
			ModelName:     modelName,
			ProviderID:    assoc.ProviderID,
			ProviderName:  providerName,
			ProviderModel: assoc.ProviderModel,
		})
	})
	return previews, nil
}

// CleanInvalid 清理无效关联，返回成功删除条数。不检查全局开关。
// 若存在删除失败，返回已成功条数与 error。
func (s *Service) CleanInvalid(ctx context.Context) (int, error) {
	data, err := s.fetchCleanData(ctx, false)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	failedCount := 0
	repos := s.repositories()
	s.forEachInvalidAssociation(
		ctx,
		data,
		func(provider *models.Provider, err error) {
			slog.Warn("failed to get provider models", "provider_id", provider.ID, "error", err)
		},
		func(assoc models.ModelWithProvider, _ *models.Provider) {
			if _, err := repos.ModelWithProvider.Delete(ctx, assoc.ID); err != nil {
				failedCount++
				slog.Warn("failed to delete invalid association", "id", assoc.ID, "error", err)
				return
			}
			removedCount++
		},
	)
	if failedCount > 0 {
		return removedCount, fmt.Errorf("auto-clean: %d removed, %d failed", removedCount, failedCount)
	}
	return removedCount, nil
}

// TriggerCleanIfEnabled 若全局开关开启则执行 CleanInvalid。
func (s *Service) TriggerCleanIfEnabled(ctx context.Context) {
	if !s.settingBool(ctx, models.SettingKeyAutoCleanOnDelete, false) {
		slog.Info("auto-clean disabled")
		return
	}
	slog.Info("auto-clean triggered")

	removedCount, err := s.CleanInvalid(ctx)
	if err != nil {
		slog.Error("auto-clean failed", "error", err, "removed", removedCount)
		return
	}
	if removedCount > 0 {
		slog.Info("auto-cleaned invalid associations", "count", removedCount)
	}
}

type associateData struct {
	allModels            []models.Model
	allProviders         []models.Provider
	existingAssociations []models.ModelWithProvider
	manualTemplateItems  []models.ModelTemplateItem
}

func (s *Service) fetchAssociateData(ctx context.Context) (associateData, error) {
	repos := s.repositories()

	allModels, err := repos.Model.List(ctx)
	if err != nil {
		return associateData{}, fmt.Errorf("get models: %w", err)
	}
	allProviders, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil {
		return associateData{}, fmt.Errorf("get providers: %w", err)
	}
	existingAssociations, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		return associateData{}, fmt.Errorf("get existing associations: %w", err)
	}
	manualTemplateItems, err := repos.ModelTemplateItem.ListAll(ctx)
	if err != nil {
		return associateData{}, fmt.Errorf("get template items: %w", err)
	}

	return associateData{
		allModels:            allModels,
		allProviders:         allProviders,
		existingAssociations: existingAssociations,
		manualTemplateItems:  manualTemplateItems,
	}, nil
}

type cleanData struct {
	allAssociations []models.ModelWithProvider
	allProviders    []models.Provider
	allModels       []models.Model
}

func (s *Service) fetchCleanData(ctx context.Context, includeModels bool) (cleanData, error) {
	repos := s.repositories()

	allAssociations, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		return cleanData{}, fmt.Errorf("get associations: %w", err)
	}
	allProviders, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil {
		return cleanData{}, fmt.Errorf("get providers: %w", err)
	}

	data := cleanData{
		allAssociations: allAssociations,
		allProviders:    allProviders,
	}
	if !includeModels {
		return data, nil
	}

	allModels, err := repos.Model.List(ctx)
	if err != nil {
		return cleanData{}, fmt.Errorf("get models: %w", err)
	}
	data.allModels = allModels
	return data, nil
}

func (s *Service) forEachMissingAssociation(
	ctx context.Context,
	data associateData,
	onProviderModelsError func(provider models.Provider, err error),
	visit func(candidate associationCandidate, key string, existingMap map[string]bool),
) {
	modelByID := indexModelsByID(data.allModels)
	existingMap := buildExistingAssociationMap(data.existingAssociations)
	templateIndex := s.buildIndex(data.allModels, data.existingAssociations, data.manualTemplateItems)

	for _, provider := range data.allProviders {
		if isProviderBlacklisted(provider) {
			continue
		}

		providerModels, err := modelsync.GetProviderModels(ctx, provider)
		if err != nil {
			if onProviderModelsError != nil {
				onProviderModelsError(provider, err)
			}
			continue
		}

		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
				model, ok := modelByID[modelID]
				if !ok {
					continue
				}
				if !allowsAutoAssociate(model) {
					continue
				}

				key := buildAssociationKey(model.ID, provider.ID, providerModel)
				if existingMap[key] {
					continue
				}

				visit(associationCandidate{
					ModelID:       model.ID,
					ModelName:     model.Name,
					ProviderID:    provider.ID,
					ProviderName:  provider.Name,
					ProviderModel: providerModel,
				}, key, existingMap)
			}
		}
	}
}

func (s *Service) forEachInvalidAssociation(
	ctx context.Context,
	data cleanData,
	onProviderModelsError func(provider *models.Provider, err error),
	visit func(assoc models.ModelWithProvider, provider *models.Provider),
) {
	providerMap := indexProvidersByID(data.allProviders)

	for _, assoc := range data.allAssociations {
		provider, providerExists := providerMap[assoc.ProviderID]
		if !providerExists {
			visit(assoc, nil)
			continue
		}

		providerModels, err := modelsync.GetProviderModels(ctx, *provider)
		if err != nil {
			if onProviderModelsError != nil {
				onProviderModelsError(provider, err)
			}
			continue
		}

		if providerModelExists(providerModels, assoc.ProviderModel) {
			continue
		}

		visit(assoc, provider)
	}
}
