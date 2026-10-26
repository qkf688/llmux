package service

import (
	"sync"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/autoassoc"
)

var (
	autoAssocOnce sync.Once
	autoAssocSvc  *autoassoc.Service
)

// GetAutoAssocService 返回自动关联服务单例（模板索引经本门面注入，避免子包循环依赖）。
func GetAutoAssocService() *autoassoc.Service {
	autoAssocOnce.Do(func() {
		autoAssocSvc = autoassoc.NewService(nil, func(
			allModels []models.Model,
			allAssociations []models.ModelWithProvider,
			manualItems []models.ModelTemplateItem,
		) autoassoc.NameMatcher {
			return BuildTemplateIndexFromData(allModels, allAssociations, manualItems)
		})
	})
	return autoAssocSvc
}

// NewAutoAssocService 创建绑定指定 repos 的自动关联服务（测试或自定义装配）。
func NewAutoAssocService(repos *repository.Repositories) *autoassoc.Service {
	return autoassoc.NewService(repos, func(
		allModels []models.Model,
		allAssociations []models.ModelWithProvider,
		manualItems []models.ModelTemplateItem,
	) autoassoc.NameMatcher {
		return BuildTemplateIndexFromData(allModels, allAssociations, manualItems)
	})
}

// ProviderWriteState 是「provider 写操作前后的状态快照」，判定要不要重算关联的输入。
// 别名自 service/autoassoc：handler 层拿类型不必再引子包。
type ProviderWriteState = autoassoc.ProviderWriteState

// DetectProviderWriteEffect 判定一次 provider 写要不要重算关联表（纯函数，无副作用）。
// 与 autoassoc.DetectProviderWriteEffect 同一份口径，经本门面转出避免调用方引子包。
func DetectProviderWriteEffect(before, after ProviderWriteState) autoassoc.ProviderWriteEffect {
	return autoassoc.DetectProviderWriteEffect(before, after)
}
