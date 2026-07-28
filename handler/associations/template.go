package associations

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
)

// SaveProviderModelToTemplate 将 ProviderModel 保存到模板项（幂等：已存在则跳过）。
func SaveProviderModelToTemplate(ctx context.Context, modelID uint, providerModel string) error {
	return saveProviderModelToTemplate(ctx, repos(), modelID, providerModel)
}

func saveProviderModelToTemplate(ctx context.Context, r *repository.Repositories, modelID uint, providerModel string) error {
	if providerModel == "" {
		return nil
	}

	count, err := r.ModelTemplateItem.CountByModelIDAndName(ctx, modelID, providerModel)
	if err != nil {
		return err
	}

	if count == 0 {
		item := models.ModelTemplateItem{
			ModelID: modelID,
			Name:    providerModel,
		}
		if err := r.ModelTemplateItem.Create(ctx, &item); err != nil {
			return err
		}
		slog.Info("auto-saved provider model to template",
			"model_id", modelID,
			"provider_model", providerModel)
	}

	return nil
}

// BatchImportExistingAssociations 批量导入现有关联到模板。
// 供 settings 开启 auto_save 时委托调用，避免双实现/空壳。
func BatchImportExistingAssociations(ctx context.Context) {
	batchImportExistingAssociations(ctx, repos())
}

func batchImportExistingAssociations(ctx context.Context, r *repository.Repositories) {
	slog.Info("starting batch import of existing associations to template")

	assocs, err := r.ModelWithProvider.ListAll(ctx)
	if err != nil {
		slog.Error("failed to fetch existing associations", "error", err)
		return
	}

	imported := 0
	skipped := 0
	failed := 0

	for _, assoc := range assocs {
		if assoc.ProviderModel == "" {
			skipped++
			continue
		}

		if err := saveProviderModelToTemplate(ctx, r, assoc.ModelID, assoc.ProviderModel); err != nil {
			slog.Warn("failed to import association to template",
				"model_id", assoc.ModelID,
				"provider_model", assoc.ProviderModel,
				"error", err)
			failed++
		} else {
			imported++
		}
	}

	slog.Info("batch import completed",
		"total", len(assocs),
		"imported", imported,
		"skipped", skipped,
		"failed", failed)
}
