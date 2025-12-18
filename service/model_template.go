package service

import (
	"slices"

	"github.com/atopos31/llmio/models"
)

// TemplateIndex 提供按名称匹配到 ModelID 列表的索引。
// 规则：任一 providerModel 命中 Model.Name / 既有关联 ProviderModel / 手动模板项 Name，即视为匹配。
type TemplateIndex struct {
	byName map[string]map[uint]struct{}
}

func BuildTemplateIndexFromData(
	allModels []models.Model,
	allAssociations []models.ModelWithProvider,
	manualItems []models.ModelTemplateItem,
) TemplateIndex {
	index := TemplateIndex{
		byName: make(map[string]map[uint]struct{}),
	}

	add := func(name string, modelID uint) {
		if name == "" || modelID == 0 {
			return
		}
		modelIDs, ok := index.byName[name]
		if !ok {
			modelIDs = make(map[uint]struct{})
			index.byName[name] = modelIDs
		}
		modelIDs[modelID] = struct{}{}
	}

	for _, m := range allModels {
		add(m.Name, m.ID)
	}
	for _, assoc := range allAssociations {
		add(assoc.ProviderModel, assoc.ModelID)
	}
	for _, item := range manualItems {
		add(item.Name, item.ModelID)
	}

	return index
}

func (idx TemplateIndex) Match(name string) []uint {
	modelIDs, ok := idx.byName[name]
	if !ok {
		return nil
	}
	ids := make([]uint, 0, len(modelIDs))
	for id := range modelIDs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

