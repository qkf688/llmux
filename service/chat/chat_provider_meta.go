package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/virtualmodel"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// CandidatePool 是「某个真实模型下的可用供应商候选集」：选路重试循环所需的全部候选数据。
// 真实路径在解析请求时构建一次；虚拟路径为每个 ordered model 各构建一次。
type CandidatePool struct {
	ModelWithProviderMap map[uint]models.ModelWithProvider
	ProviderMap          map[uint]models.Provider
	WeightItems          map[uint]int
	PriorityItems        map[uint]int
}

// buildCandidatePool 由关联列表装配候选池。err 只上抛不处理——真实路径据此中断请求、
// 虚拟路径据此跳过当前真实模型，两种分歧留在各自调用点（本函数不含路径判断）。
func buildCandidatePool(ctx context.Context, modelWithProviders []models.ModelWithProvider) (CandidatePool, error) {
	providerMap, err := buildProviderMapByModelProviders(ctx, modelWithProviders)
	if err != nil {
		return CandidatePool{}, err
	}
	weightItems, priorityItems := buildSelectionItemsByModelProviders(modelWithProviders, providerMap)
	return CandidatePool{
		ModelWithProviderMap: lo.KeyBy(modelWithProviders, func(mp models.ModelWithProvider) uint { return mp.ID }),
		ProviderMap:          providerMap,
		WeightItems:          weightItems,
		PriorityItems:        priorityItems,
	}, nil
}

type ProvidersWithMeta struct {
	// CandidatePool 真实路径的候选供应商集合；虚拟路径为零值——其候选池在
	// balanceChatVirtual 循环内按当前 ordered model 逐个构建，不经本字段。
	CandidatePool CandidatePool

	IOLog bool

	// Model 当前请求关联的真实模型：真实路径为查询到的 model；虚拟路径不设置本字段
	//（虚拟分支在 balanceChatVirtual 循环内经 retryLoopInput.Model 注入正在尝试的 ordered model）。
	// 供裁剪 thinking 字段时解析 ModelWithProvider.SupportsThinkingResolved（关联 override → model 继承）。
	Model *models.Model

	// 虚拟模型相关字段
	IsVirtualModel    bool                            // 是否是虚拟模型
	VirtualModelID    uint                            // 虚拟模型ID
	VirtualModelName  string                          // 虚拟模型名称
	VirtualStrategy   string                          // 虚拟模型策略
	OrderedRealModels []virtualmodel.OrderedRealModel // 有序的真实模型列表
}

func ProvidersWithMetaBymodelsName(ctx context.Context, style string, before Before, reqMeta models.ReqMeta) (*ProvidersWithMeta, error) {
	// 首先检查是否是虚拟模型
	virtualModel, err := repos().VirtualModel.GetEnabledByName(ctx, before.Model)
	if err == nil {
		// 是虚拟模型，使用虚拟模型服务获取有序的真实模型列表
		slog.Info("request virtual model", "virtual_model", virtualModel.Name, "strategy", virtualModel.Strategy)

		orderedModels, err := virtualmodel.Default().SelectRealModelsOrdered(ctx, virtualModel)
		if err != nil {
			savePreProviderFailureLog(ctx, reqMeta, style, preProviderFailureLog{
				RequestedModel: before.Model,
				// RealModelName unknown: virtual selection failed before any real model was chosen.
				Error: fmt.Sprintf("virtual model selection failed: %v", err),
			})
			return nil, fmt.Errorf("failed to select real models from virtual model: %w", err)
		}

		// 列表必非空，可直接取首元素：SelectRealModelsOrdered 的候选池在装配阶段
		// 已保证至少一条可用映射（无启用映射 / 真实模型缺失 / 全被拉黑 均在此之前
		// 报错），各策略再按 mapping 逐一构建，不存在中途变空的路径。
		slog.Info("selected ordered real models from virtual model",
			"virtual_model", virtualModel.Name,
			"count", len(orderedModels),
			"first_model", orderedModels[0].Model.Name)

		// 应用虚拟模型的配置到所有真实模型：IOLog 仍为 per-model 语义；
		// 超时/重试已全局化（见 chat_balance / chat_balance_virtual），不再覆盖。
		for i := range orderedModels {
			if virtualModel.IOLog != nil {
				orderedModels[i].Model.IOLog = virtualModel.IOLog
			}
		}

		firstModel := orderedModels[0].Model
		if firstModel.IOLog == nil {
			firstModel.IOLog = new(bool)
		}

		return &ProvidersWithMeta{
			IOLog: *firstModel.IOLog,
			// 虚拟模型相关字段
			IsVirtualModel:    true,
			VirtualModelID:    virtualModel.ID,
			VirtualModelName:  virtualModel.Name,
			VirtualStrategy:   virtualModel.Strategy,
			OrderedRealModels: orderedModels,
		}, nil
	}

	// 不是虚拟模型，按原有逻辑处理真实模型
	model, err := repos().Model.GetByName(ctx, before.Model)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// 404 而非 500：模型不存在是配置态失败，重试不可能变好（对齐 OpenAI
		// 「model not found」口径，客户端不重试）；模型名是调用方自有输入，可拼进文案。
		savePreProviderFailureLog(ctx, reqMeta, style, preProviderFailureLog{
			RequestedModel: before.Model,
			RealModelName:  before.Model,
			Error:          err.Error(),
		})
		return nil, newClientRequestError(http.StatusNotFound, "not found model "+before.Model)
	}

	modelWithProviders, err := queryEnabledModelProviders(ctx, model.ID, before)
	if err != nil {
		return nil, err
	}

	if len(modelWithProviders) == 0 {
		reason := noUsableProviderReason(ctx, before.Model)
		savePreProviderFailureLog(ctx, reqMeta, style, preProviderFailureLog{
			RequestedModel: before.Model,
			RealModelName:  model.Name,
			Error:          reason,
		})
		// 404 而非 500：无启用关联（没关联/全停用/能力筛光）同为配置态失败，
		// 重试不可能变好；文案复用落库原因，带能力筛选提示与模型名。
		return nil, newClientRequestError(http.StatusNotFound, reason)
	}

	// 不再按 style 过滤供应商，因为现在支持格式转换
	// 客户端可以使用任意格式请求任意类型的供应商
	pool, err := buildCandidatePool(ctx, modelWithProviders)
	if err != nil {
		return nil, err
	}

	// 按优先级排序供应商（用于日志输出）
	type providerPriority struct {
		ID       uint
		Priority int
	}
	var sortedProviders []providerPriority
	for id, priority := range pool.PriorityItems {
		sortedProviders = append(sortedProviders, providerPriority{ID: id, Priority: priority})
	}
	sort.Slice(sortedProviders, func(i, j int) bool {
		return sortedProviders[i].Priority > sortedProviders[j].Priority
	})
	slog.Debug("providers sorted by priority", "order", sortedProviders)

	if model.IOLog == nil {
		model.IOLog = new(bool)
	}

	return &ProvidersWithMeta{
		CandidatePool: pool,
		IOLog:         *model.IOLog,
		Model:         model,
	}, nil
}

// queryEnabledModelProviders 按请求能力需求筛选该模型下已启用的关联。
func queryEnabledModelProviders(ctx context.Context, modelID uint, before Before) ([]models.ModelWithProvider, error) {
	var caps repository.ModelCapabilityFilter
	if getStrictCapabilityMatch(ctx) {
		caps = repository.ModelCapabilityFilter{
			ToolCall:         before.toolCall,
			StructuredOutput: before.structuredOutput,
			Image:            before.image,
		}
	}

	return repos().ModelWithProvider.ListEnabledByModelID(ctx, modelID, caps)
}

// noUsableProviderReason 说明「真实模型查不到可用供应商」的原因。
//
// 必须点明能力筛选：严格能力匹配开启时，「关联页显示已启用」与「本请求无可用供应商」
// 可以同时成立——请求自带的工具调用 / 结构化输出 / 图片需求会把能力不满足的关联筛掉。
// 不带这句提示时，该场景表现为「明明启用了却说没有供应商」，会被当成系统 bug 排查。
func noUsableProviderReason(ctx context.Context, modelName string) string {
	if getStrictCapabilityMatch(ctx) {
		return fmt.Sprintf("no enabled provider for model %q (strict capability match is on: associations may be filtered out by tool_call / structured_output / image requirements)", modelName)
	}
	return fmt.Sprintf("no enabled provider for model %q", modelName)
}

func buildProviderMapByModelProviders(ctx context.Context, modelWithProviders []models.ModelWithProvider) (map[uint]models.Provider, error) {
	providerIDs := lo.Map(modelWithProviders, func(mp models.ModelWithProvider, _ int) uint { return mp.ProviderID })
	providersList, err := repos().Provider.ListByIDs(ctx, providerIDs)
	if err != nil {
		return nil, err
	}
	return lo.KeyBy(providersList, func(p models.Provider) uint { return p.ID }), nil
}

func buildSelectionItemsByModelProviders(modelWithProviders []models.ModelWithProvider, providerMap map[uint]models.Provider) (map[uint]int, map[uint]int) {
	weightItems := make(map[uint]int)
	priorityItems := make(map[uint]int)
	for _, mp := range modelWithProviders {
		if _, ok := providerMap[mp.ProviderID]; !ok {
			continue
		}
		weightItems[mp.ID] = mp.Weight
		priorityItems[mp.ID] = mp.Priority
	}
	return weightItems, priorityItems
}
