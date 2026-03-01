package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// TemplateIndex 提供按名称匹配到 ModelID 列表的索引。
// 规则：任一 providerModel 命中 Model.Name / 既有关联 ProviderModel / 手动模板项 Name，即视为匹配。
type TemplateIndex struct {
	byName        map[string]map[uint]struct{}
	fuzzyMatch    bool
	separators    []string
	fuzzySuffixes []string
}

func BuildTemplateIndexFromData(
	allModels []models.Model,
	allAssociations []models.ModelWithProvider,
	manualItems []models.ModelTemplateItem,
) TemplateIndex {
	ctx := context.Background()
	index := TemplateIndex{
		byName: make(map[string]map[uint]struct{}),
	}

	fuzzyMatch, separators, suffixes := loadTemplateFuzzySettings(ctx)
	index.fuzzyMatch = fuzzyMatch
	index.separators = separators
	index.fuzzySuffixes = suffixes

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
	// 精确匹配优先
	modelIDs, ok := idx.byName[name]
	if ok {
		ids := make([]uint, 0, len(modelIDs))
		for id := range modelIDs {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		return ids
	}

	// 如果启用模糊匹配，尝试模糊后缀匹配
	if idx.fuzzyMatch {
		return idx.fuzzyMatchBySuffix(name)
	}

	return nil
}

// fuzzyMatchBySuffix 模糊后缀匹配：检查 name 是否匹配 "模板名 + 分隔符 + 后缀" 的模式
func (idx TemplateIndex) fuzzyMatchBySuffix(name string) []uint {
	resultSet := make(map[uint]struct{})

	// 遍历所有模板名称
	for templateName, modelIDs := range idx.byName {
		// 检查 name 是否以 templateName 开头
		if !strings.HasPrefix(name, templateName) {
			continue
		}

		// 提取后缀部分
		remainder := name[len(templateName):]
		if len(remainder) == 0 {
			continue
		}

		// 检查是否匹配 "分隔符 + 后缀" 模式
		if idx.matchesSeparatorAndSuffix(remainder) {
			for id := range modelIDs {
				resultSet[id] = struct{}{}
			}
		}
	}

	if len(resultSet) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(resultSet))
	for id := range resultSet {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// matchesSeparatorAndSuffix 检查字符串是否匹配 "分隔符 + 后缀" 模式
func (idx TemplateIndex) matchesSeparatorAndSuffix(s string) bool {
	for _, sep := range idx.separators {
		if strings.HasPrefix(s, sep) {
			suffix := s[len(sep):]
			for _, allowedSuffix := range idx.fuzzySuffixes {
				if suffix == allowedSuffix {
					return true
				}
			}
		}
	}
	return false
}

// loadTemplateFuzzySettings 从数据库加载模板模糊匹配设置
func loadTemplateFuzzySettings(ctx context.Context) (bool, []string, []string) {
	db := models.DB
	if db == nil {
		return false, nil, nil
	}

	// 加载模糊匹配开关
	fuzzyEnabled := false
	if setting, err := gorm.G[models.Setting](db).Where("key = ?", models.SettingKeyTemplateFuzzyMatchEnabled).First(ctx); err == nil {
		fuzzyEnabled = setting.Value == "true"
	}

	if !fuzzyEnabled {
		return false, nil, nil
	}

	// 加载分隔符
	separators := []string{":", "-"}
	if setting, err := gorm.G[models.Setting](db).Where("key = ?", models.SettingKeyTemplateFuzzyMatchSeparators).First(ctx); err == nil {
		var seps []string
		if err := json.Unmarshal([]byte(setting.Value), &seps); err == nil && len(seps) > 0 {
			separators = seps
		}
	}

	// 加载后缀关键词
	suffixes := []string{"free"}
	if setting, err := gorm.G[models.Setting](db).Where("key = ?", models.SettingKeyTemplateFuzzyMatchSuffixes).First(ctx); err == nil {
		var suffs []string
		if err := json.Unmarshal([]byte(setting.Value), &suffs); err == nil && len(suffs) > 0 {
			suffixes = suffs
		}
	}

	return fuzzyEnabled, separators, suffixes
}
