package modelapi

import (
	"slices"
	"strings"

	"github.com/qkf688/llmux/models"
)

func buildModelTemplateResponse(
	model models.Model,
	associations []models.ModelWithProvider,
	manualItems []models.ModelTemplateItem,
) ModelTemplateResponse {
	sourceSetByName := make(map[string]map[string]struct{})
	addSource := func(name, source string) {
		if name == "" || source == "" {
			return
		}
		set, ok := sourceSetByName[name]
		if !ok {
			set = make(map[string]struct{})
			sourceSetByName[name] = set
		}
		set[source] = struct{}{}
	}

	addSource(model.Name, "model_name")
	for _, assoc := range associations {
		addSource(assoc.ProviderModel, "association")
	}
	for _, item := range manualItems {
		addSource(item.Name, "manual")
	}

	items := make([]ModelTemplateItemResponse, 0, len(sourceSetByName))
	for name, sources := range sourceSetByName {
		sourceList := make([]string, 0, len(sources))
		for src := range sources {
			sourceList = append(sourceList, src)
		}
		slices.Sort(sourceList)
		items = append(items, ModelTemplateItemResponse{
			Name:    name,
			Sources: sourceList,
		})
	}
	slices.SortFunc(items, func(a, b ModelTemplateItemResponse) int {
		return strings.Compare(a.Name, b.Name)
	})

	return ModelTemplateResponse{
		ModelID:   model.ID,
		ModelName: model.Name,
		Items:     items,
	}
}
