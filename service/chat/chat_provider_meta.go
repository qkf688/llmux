package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/atopos31/llmio/service/virtualmodel"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type ProvidersWithMeta struct {
	ModelWithProviderMap map[uint]models.ModelWithProvider
	WeightItems          map[uint]int
	PriorityItems        map[uint]int
	ProviderMap          map[uint]models.Provider
	MaxRetry             int
	TimeOut              int
	IOLog                bool

	// 虚拟模型相关字段
	IsVirtualModel    bool                            // 是否是虚拟模型
	VirtualModelID    uint                            // 虚拟模型ID
	VirtualModelName  string                          // 虚拟模型名称
	VirtualStrategy   string                          // 虚拟模型策略
	OrderedRealModels []virtualmodel.OrderedRealModel // 有序的真实模型列表
}

func ProvidersWithMetaBymodelsName(ctx context.Context, style string, before Before) (*ProvidersWithMeta, error) {
	// 首先检查是否是虚拟模型
	virtualModel, err := repos().VirtualModel.GetEnabledByName(ctx, before.Model)
	if err == nil {
		// 是虚拟模型，使用虚拟模型服务获取有序的真实模型列表
		slog.Info("request virtual model", "virtual_model", virtualModel.Name, "strategy", virtualModel.Strategy)

		orderedModels, err := virtualmodel.Default().SelectRealModelsOrdered(ctx, virtualModel)
		if err != nil {
			if _, err := SaveChatLog(ctx, models.ChatLog{
				Name: before.Model,
				// RealModelName unknown: virtual selection failed before any real model was chosen.
				Status: "error",
				Style:  style,
				Error:  fmt.Sprintf("virtual model selection failed: %v", err),
			}); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("failed to select real models from virtual model: %w", err)
		}

		if len(orderedModels) == 0 {
			return nil, errors.New("no real models found for virtual model")
		}

		slog.Info("selected ordered real models from virtual model",
			"virtual_model", virtualModel.Name,
			"count", len(orderedModels),
			"first_model", orderedModels[0].Model.Name)

		// 应用虚拟模型的配置到所有真实模型
		for i := range orderedModels {
			if virtualModel.MaxRetry > 0 {
				orderedModels[i].Model.MaxRetry = virtualModel.MaxRetry
			}
			if virtualModel.TimeOut > 0 {
				orderedModels[i].Model.TimeOut = virtualModel.TimeOut
			}
			if virtualModel.IOLog != nil {
				orderedModels[i].Model.IOLog = virtualModel.IOLog
			}
		}

		firstModel := orderedModels[0].Model
		if firstModel.IOLog == nil {
			firstModel.IOLog = new(bool)
		}

		return &ProvidersWithMeta{
			ModelWithProviderMap: map[uint]models.ModelWithProvider{},
			WeightItems:          map[uint]int{},
			PriorityItems:        map[uint]int{},
			ProviderMap:          map[uint]models.Provider{},
			MaxRetry:             firstModel.MaxRetry,
			TimeOut:              firstModel.TimeOut,
			IOLog:                *firstModel.IOLog,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, err := SaveChatLog(ctx, models.ChatLog{
				Name:          before.Model,
				RealModelName: before.Model,
				Status:        "error",
				Style:         style,
				Error:         err.Error(),
			}); err != nil {
				return nil, err
			}
			return nil, errors.New("not found model " + before.Model)
		}
		return nil, err
	}

	modelWithProviders, err := queryEnabledModelProviders(ctx, model.ID, before)
	if err != nil {
		return nil, err
	}

	if len(modelWithProviders) == 0 {
		return nil, errors.New("not provider for model " + before.Model)
	}

	modelWithProviderMap := lo.KeyBy(modelWithProviders, func(mp models.ModelWithProvider) uint { return mp.ID })

	// 不再按 style 过滤供应商，因为现在支持格式转换
	// 客户端可以使用任意格式请求任意类型的供应商
	providerMap, err := buildProviderMapByModelProviders(ctx, modelWithProviders)
	if err != nil {
		return nil, err
	}

	weightItems, priorityItems := buildSelectionItemsByModelProviders(modelWithProviders, providerMap)

	// 按优先级排序供应商（用于日志输出）
	type providerPriority struct {
		ID       uint
		Priority int
	}
	var sortedProviders []providerPriority
	for id, priority := range priorityItems {
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
		ModelWithProviderMap: modelWithProviderMap,
		WeightItems:          weightItems,
		PriorityItems:        priorityItems,
		ProviderMap:          providerMap,
		MaxRetry:             model.MaxRetry,
		TimeOut:              model.TimeOut,
		IOLog:                *model.IOLog,
	}, nil
}

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
