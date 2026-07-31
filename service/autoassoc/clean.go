package autoassoc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/modelsync"
)

// CleanInvalid 清理无效关联，返回执行结果。不检查全局开关。
// 部分失败不返回 error，仅通过 Result.Failed 观测。
func (s *Service) CleanInvalid(ctx context.Context) (Result, error) {
	data, err := s.fetchCleanData(ctx, false)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	repos := s.repositories()
	s.forEachInvalidAssociation(
		ctx,
		data,
		func(provider *models.Provider, err error) {
			slog.Warn("failed to get provider models", "provider_id", provider.ID, "error", err)
		},
		func(assoc models.ModelWithProvider, _ *models.Provider) {
			if _, err := repos.ModelWithProvider.Delete(ctx, assoc.ID); err != nil {
				result.Failed++
				slog.Warn("failed to delete invalid association", "id", assoc.ID, "error", err)
				return
			}
			result.Success++
		},
	)
	return result, nil
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
