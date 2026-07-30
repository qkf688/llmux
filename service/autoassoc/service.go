package autoassoc

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
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

// Result 是 Associate / CleanInvalid 的执行结果。
// Success = 成功创建（Associate）或成功删除（CleanInvalid）的条数；
// Failed = 逐条写库失败数。部分失败不作为 error 返回，仅通过 Failed 字段观测。
type Result struct {
	Success int
	Failed  int
}

// LogResult 统一记录 Associate / CleanInvalid 的执行结果，消除 Trigger* 与 ActionHooks 闭包间的日志样板重复。
func LogResult(label string, result Result, err error) {
	if err != nil {
		slog.Error(label+" failed", "error", err)
		return
	}
	if result.Success > 0 {
		slog.Info(label, "count", result.Success)
	}
	if result.Failed > 0 {
		slog.Warn(label+" partial failure", "failed", result.Failed)
	}
}

// PreviewAssociate 预览将要创建的缺失关联（跳过黑名单与已存在 key）。
// 手动预览不尊重 Model.AutoAssociate——用户关掉自动关联只应拦住后台同步，不应隐藏手动入口。
func (s *Service) PreviewAssociate(ctx context.Context) ([]Preview, error) {
	data, err := s.fetchAssociateData(ctx)
	if err != nil {
		return nil, err
	}

	previews := make([]Preview, 0)
	s.forEachMissingAssociation(ctx, data, nil, nil, func(candidate associationCandidate, _ string, _ map[string]bool) {
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

// Associate 执行自动关联（尊重 Model.AutoAssociate），返回执行结果。
// 不检查全局开关（由调用方决定是否触发）。部分失败不返回 error，仅通过 Result.Failed 观测。
func (s *Service) Associate(ctx context.Context) (Result, error) {
	return s.associate(ctx, skipAutoAssociate)
}

// AssociateAll 执行手动关联（绕过 Model.AutoAssociate 门控），返回执行结果。
// 供 HTTP 手动「一键关联」入口使用——用户关掉自动关联只应拦住后台同步，不应封锁手动操作。
func (s *Service) AssociateAll(ctx context.Context) (Result, error) {
	return s.associate(ctx, nil)
}

// TriggerAssociateIfEnabled 若全局开关开启则执行 Associate（供 provider CRUD / 旁路使用）。
func (s *Service) TriggerAssociateIfEnabled(ctx context.Context) {
	if !s.settingBool(ctx, models.SettingKeyAutoAssociateOnAdd, false) {
		slog.Info("auto-associate disabled")
		return
	}
	slog.Info("auto-associate triggered")
	result, err := s.Associate(ctx)
	LogResult("auto-associated models", result, err)
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

// TriggerCleanIfEnabled 若全局开关开启则执行 CleanInvalid。
func (s *Service) TriggerCleanIfEnabled(ctx context.Context) {
	if !s.settingBool(ctx, models.SettingKeyAutoCleanOnDelete, false) {
		slog.Info("auto-clean disabled")
		return
	}
	slog.Info("auto-clean triggered")
	result, err := s.CleanInvalid(ctx)
	LogResult("auto-cleaned invalid associations", result, err)
}
