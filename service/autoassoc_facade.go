package service

import (
	"sync"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/atopos31/llmio/service/autoassoc"
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
