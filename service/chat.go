package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"sort"
	"strconv"
	"time"

	"github.com/atopos31/llmio/balancer"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func BalanceChat(ctx context.Context, start time.Time, style string, before Before, providersWithMeta ProvidersWithMeta, reqMeta models.ReqMeta) (*http.Response, uint, error) {
	slog.Info("request", "model", before.Model, "stream", before.Stream, "tool_call", before.toolCall, "structured_output", before.structuredOutput, "image", before.image)

	providerMap := providersWithMeta.ProviderMap
	weightItems := providersWithMeta.WeightItems
	priorityItems := providersWithMeta.PriorityItems

	// 收集重试过程中的err日志
	retryLog := make(chan models.ChatLog, providersWithMeta.MaxRetry)
	defer close(retryLog)

	go RecordRetryLog(context.Background(), retryLog, providersWithMeta.ModelWithProviderMap)

	// 注意：这里我们需要在循环中为每个provider创建带代理的client
	// 所以先移除这行，在循环内部创建

	timer := time.NewTimer(time.Second * time.Duration(providersWithMeta.TimeOut))
	defer timer.Stop()
	for retry := range providersWithMeta.MaxRetry {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-timer.C:
			return nil, 0, errors.New("retry time out")
		default:
			// 根据优先级和权重选择供应商
			id, err := selectByPriorityAndWeight(weightItems, priorityItems)
			if err != nil {
				return nil, 0, err
			}

			modelWithProvider, ok := providersWithMeta.ModelWithProviderMap[*id]
			if !ok {
				// 数据不一致，移除该模型避免下次重复命中
				delete(weightItems, *id)
				continue
			}

			provider := providerMap[modelWithProvider.ProviderID]

			// 使用供应商的实际类型创建 provider 实例，而不是客户端格式
			chatModel, err := providers.New(provider.Type, provider.Config, provider.Proxy)
			if err != nil {
				return nil, 0, err
			}

			// 为当前provider创建带代理的client
			// 使用完整的超时时间,特别是对于工具调用场景需要更长的等待时间
			client := providers.GetClientWithProxy(time.Second*time.Duration(providersWithMeta.TimeOut), chatModel.GetProxy())

			slog.Info("using provider", "provider", provider.Name, "model", modelWithProvider.ProviderModel, "proxy", chatModel.GetProxy())

			log := models.ChatLog{
				Name:          before.Model,
				ProviderModel: modelWithProvider.ProviderModel,
				ProviderName:  provider.Name,
				Status:        "success",
				Style:         style,
				UserAgent:     reqMeta.UserAgent,
				RemoteIP:      reqMeta.RemoteIP,
				ChatIO:        providersWithMeta.IOLog,
				Retry:         retry,
				ProxyTime:     time.Since(start),
			}
			// 根据请求原始请求头 是否透传请求头 自定义请求头 构建新的请求头
			withHeader := false
			if modelWithProvider.WithHeader != nil {
				withHeader = *modelWithProvider.WithHeader
			}
			header := buildHeaders(reqMeta.Header, withHeader, modelWithProvider.CustomerHeaders, before.Stream)

			reqStart := time.Now()
			trace := &httptrace.ClientTrace{
				GotFirstResponseByte: func() {
					slog.Debug("first response byte received", "response_time", time.Since(reqStart))
				},
			}

			// 判断是否需要格式转换
			// 当客户端格式与供应商类型一致时，直接透传原始请求体
			var requestBody []byte
			if style == provider.Type {
				// 直接透传，不进行格式转换
				slog.Debug("passthrough mode", "client_type", style, "provider_type", provider.Type)
				requestBody = before.raw
			} else {
				// 需要格式转换
				slog.Debug("transform mode", "client_type", style, "provider_type", provider.Type)
				tm := NewTransformerManager(style, provider.Type)
				convertedBody, err := tm.ProcessRequest(ctx, before.raw)
				if err != nil {
					retryLog <- log.WithError(fmt.Errorf("transform request error: %v", err))
					delete(weightItems, *id)
					continue
				}
				requestBody = convertedBody
			}

			req, err := chatModel.BuildReq(httptrace.WithClientTrace(ctx, trace), header, modelWithProvider.ProviderModel, requestBody)
			if err != nil {
				retryLog <- log.WithError(err)
				// 构建请求失败 移除待选
				delete(weightItems, *id)
				continue
			}

			// 提前创建日志记录,确保所有请求都被记录
			logId, err := SaveChatLog(ctx, log)
			if err != nil {
				slog.Error("failed to create log before request", "error", err)
				return nil, 0, err
			}

			res, err := client.Do(req)
			if err != nil {
				// 更新日志状态为错误
				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
					Status: "error",
					Error:  err.Error(),
				}); updateErr != nil {
					slog.Error("failed to update log status", "error", updateErr)
				}
				// 请求失败 移除待选
				delete(weightItems, *id)
				delete(priorityItems, *id)
				continue
			}

			if res.StatusCode != http.StatusOK {
				byteBody, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("read body error", "error", err)
				}
				// 更新日志状态为错误
				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
					Status: "error",
					Error:  fmt.Sprintf("status: %d, body: %s", res.StatusCode, string(byteBody)),
				}); updateErr != nil {
					slog.Error("failed to update log status", "error", updateErr)
				}

				if res.StatusCode == http.StatusTooManyRequests {
					// 达到RPM限制 降低权重
					weightItems[*id] -= weightItems[*id] / 3
				} else {
					// 非RPM限制 移除待选
					delete(weightItems, *id)
					delete(priorityItems, *id)
				}
				res.Body.Close()
				continue
			}

			// 判断是否需要响应格式转换
			// 当客户端格式与供应商类型一致时，直接透传响应
			if style != provider.Type {
				// 需要格式转换
				tm := NewTransformerManager(style, provider.Type)
				convertedRes, err := tm.ProcessResponse(res)
				if err != nil {
					retryLog <- log.WithError(fmt.Errorf("transform response error: %v", err))
					res.Body.Close()
					delete(weightItems, *id)
					continue
				}
				res = convertedRes
			} else {
				// 直接透传响应，不进行格式转换
				slog.Debug("passthrough response", "client_type", style, "provider_type", provider.Type)
			}

			applySuccessAdjustments(ctx, *id)
			return res, logId, nil
		}
	}

	return nil, 0, errors.New("maximum retry attempts reached")
}

// selectByPriorityAndWeight 根据优先级和权重选择供应商
// 优先选择优先级高的，优先级相同时按权重随机选择
func selectByPriorityAndWeight(weightItems map[uint]int, priorityItems map[uint]int) (*uint, error) {
	if len(weightItems) == 0 {
		return nil, fmt.Errorf("no provide items")
	}

	// 找到最高优先级
	maxPriority := -1
	for id := range weightItems {
		if priority, ok := priorityItems[id]; ok && priority > maxPriority {
			maxPriority = priority
		}
	}

	// 筛选出最高优先级的供应商
	highPriorityItems := make(map[uint]int)
	for id, weight := range weightItems {
		if priority, ok := priorityItems[id]; ok && priority == maxPriority {
			highPriorityItems[id] = weight
		}
	}

	// 在最高优先级的供应商中按权重随机选择
	if len(highPriorityItems) > 0 {
		return balancer.WeightedRandom(highPriorityItems)
	}

	// 如果没有优先级信息，回退到原来的权重选择
	return balancer.WeightedRandom(weightItems)
}

func RecordRetryLog(ctx context.Context, retryLog chan models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	for log := range retryLog {
		if _, err := SaveChatLog(ctx, log); err != nil {
			slog.Error("save chat log error", "error", err)
		}
		// 当调用失败时，检查并应用权重衰减和优先级衰减
		if log.Status == "error" {
			applyWeightDecay(ctx, log, modelWithProviderMap)
			applyPriorityDecay(ctx, log, modelWithProviderMap)
		}
	}
}

// applyWeightDecay 应用权重衰减
func applyWeightDecay(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	// 检查是否开启自动权重衰减
	if !getAutoWeightDecay(ctx) {
		return
	}

	// 查找对应的 ModelWithProvider
	for id, mwp := range modelWithProviderMap {
		// 获取供应商信息以匹配日志
		provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", mwp.ProviderID).First(ctx)
		if err != nil {
			continue
		}
		if provider.Name == log.ProviderName && mwp.ProviderModel == log.ProviderModel {
			applyWeightDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
			break
		}
	}
}

// getAutoWeightDecay 获取自动权重衰减开关
func getAutoWeightDecay(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecay).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}

// getAutoWeightDecayStep 获取自动权重衰减步长
func getAutoWeightDecayStep(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayStep).
		First(ctx)
	if err != nil {
		return 1 // 默认步长1
	}
	step, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 1
	}
	return step
}

// applyPriorityDecay 应用优先级衰减
func applyPriorityDecay(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	// 检查是否开启自动优先级衰减
	if !getAutoPriorityDecay(ctx) {
		return
	}

	// 查找对应的 ModelWithProvider
	for id, mwp := range modelWithProviderMap {
		// 获取供应商信息以匹配日志
		provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", mwp.ProviderID).First(ctx)
		if err != nil {
			continue
		}
		if provider.Name == log.ProviderName && mwp.ProviderModel == log.ProviderModel {
			applyPriorityDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
			break
		}
	}
}

// getAutoPriorityDecay 获取自动优先级衰减开关
func getAutoPriorityDecay(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecay).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}

// getAutoPriorityDecayStep 获取自动优先级衰减步长
func getAutoPriorityDecayStep(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayStep).
		First(ctx)
	if err != nil {
		return 1 // 默认步长1
	}
	step, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 1
	}
	return step
}

// getAutoPriorityDecayThreshold 获取自动优先级衰减阈值
func getAutoPriorityDecayThreshold(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayThreshold).
		First(ctx)
	if err != nil {
		return 90 // 默认阈值90
	}
	threshold, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 90
	}
	return threshold
}

func RecordLog(ctx context.Context, reqStart time.Time, reader io.ReadCloser, processer Processer, logId uint, before Before, ioLog bool) {
	recordFunc := func() error {
		defer reader.Close()

		log, output, err := processer(ctx, reader, before.Stream, reqStart)
		if err != nil {
			slog.Error("processer error", "log_id", logId, "error", err)
			// 更新日志状态为错误
			if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
				Status: "error",
				Error:  fmt.Sprintf("processer error: %v", err),
			}); updateErr != nil {
				slog.Error("failed to update log status on processer error", "log_id", logId, "error", updateErr)
			}
			return err
		}

		// 更新日志记录
		if _, err := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, *log); err != nil {
			slog.Error("failed to update log", "log_id", logId, "error", err)
			return err
		}

		// 只有在启用 IO 日志时才记录输入输出
		if ioLog {
			if err := gorm.G[models.ChatIO](models.DB).Create(ctx, &models.ChatIO{
				Input:       string(before.raw),
				LogId:       logId,
				OutputUnion: *output,
			}); err != nil {
				slog.Error("failed to create chat io", "log_id", logId, "error", err)
				return err
			}
		}
		return nil
	}
	if err := recordFunc(); err != nil {
		slog.Error("record log error", "log_id", logId, "error", err)
	}
}

func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	if err := gorm.G[models.ChatLog](models.DB).Create(ctx, &log); err != nil {
		return 0, err
	}
	// 异步执行日志清理，避免阻塞主流程
	go cleanupLogsIfNeeded()
	return log.ID, nil
}

// cleanupLogsIfNeeded 检查并清理超出保留条数的日志
func cleanupLogsIfNeeded() {
	ctx := context.Background()

	// 获取日志保留条数设置
	retentionCount := getLogRetentionCount(ctx)
	if retentionCount <= 0 {
		return // 0 表示不限制
	}

	// 获取总日志数
	var total int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count logs for cleanup", "error", err)
		return
	}

	// 如果日志数超过保留条数，删除多余的
	if int(total) > retentionCount {
		deleteCount := int(total) - retentionCount

		// 获取需要删除的日志ID（最旧的）
		var logsToDelete []models.ChatLog
		if err := models.DB.Model(&models.ChatLog{}).
			Order("id ASC").
			Limit(deleteCount).
			Find(&logsToDelete).Error; err != nil {
			slog.Error("failed to find logs to delete", "error", err)
			return
		}

		// 提取ID列表
		ids := make([]uint, len(logsToDelete))
		for i, log := range logsToDelete {
			ids[i] = log.ID
		}

		// 删除对应的ChatIO记录
		if _, err := gorm.G[models.ChatIO](models.DB).
			Where("log_id IN ?", ids).
			Delete(ctx); err != nil {
			slog.Error("failed to delete chat io records", "error", err)
		}

		// 删除日志记录
		if _, err := gorm.G[models.ChatLog](models.DB).
			Where("id IN ?", ids).
			Delete(ctx); err != nil {
			slog.Error("failed to delete logs", "error", err)
			return
		}

		slog.Info("auto cleaned up excess logs", "deleted", deleteCount, "retention", retentionCount)
	}
}

// getLogRetentionCount 获取日志保留条数设置
func getLogRetentionCount(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRetentionCount).
		First(ctx)
	if err != nil {
		return 0 // 默认不限制
	}
	count, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 0
	}
	return count
}

func buildHeaders(source http.Header, withHeader bool, customHeaders map[string]string, stream bool) http.Header {
	header := http.Header{}
	if withHeader {
		header = source.Clone()
	}

	if stream {
		header.Set("X-Accel-Buffering", "no")
	}

	header.Del("Authorization")
	header.Del("X-Api-Key")

	for key, value := range customHeaders {
		header.Set(key, value)
	}

	return header
}

type ProvidersWithMeta struct {
	ModelWithProviderMap map[uint]models.ModelWithProvider
	WeightItems          map[uint]int
	PriorityItems        map[uint]int
	ProviderMap          map[uint]models.Provider
	MaxRetry             int
	TimeOut              int
	IOLog                bool
}

func ProvidersWithMetaBymodelsName(ctx context.Context, style string, before Before) (*ProvidersWithMeta, error) {
	model, err := gorm.G[models.Model](models.DB).Where("name = ?", before.Model).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, err := SaveChatLog(ctx, models.ChatLog{
				Name:   before.Model,
				Status: "error",
				Style:  style,
				Error:  err.Error(),
			}); err != nil {
				return nil, err
			}
			return nil, errors.New("not found model " + before.Model)
		}
		return nil, err
	}

	modelWithProviderChain := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", model.ID).Where("status = ?", true)

	// 检查是否启用严格能力匹配
	strictCapabilityMatch := getStrictCapabilityMatch(ctx)

	if strictCapabilityMatch {
		if before.toolCall {
			modelWithProviderChain = modelWithProviderChain.Where("tool_call = ?", true)
		}

		if before.structuredOutput {
			modelWithProviderChain = modelWithProviderChain.Where("structured_output = ?", true)
		}

		if before.image {
			modelWithProviderChain = modelWithProviderChain.Where("image = ?", true)
		}
	}

	modelWithProviders, err := modelWithProviderChain.Find(ctx)
	if err != nil {
		return nil, err
	}

	if len(modelWithProviders) == 0 {
		return nil, errors.New("not provider for model " + before.Model)
	}

	modelWithProviderMap := lo.KeyBy(modelWithProviders, func(mp models.ModelWithProvider) uint { return mp.ID })

	// 不再按 style 过滤供应商，因为现在支持格式转换
	// 客户端可以使用任意格式请求任意类型的供应商
	providers, err := gorm.G[models.Provider](models.DB).
		Where("id IN ?", lo.Map(modelWithProviders, func(mp models.ModelWithProvider, _ int) uint { return mp.ProviderID })).
		Find(ctx)
	if err != nil {
		return nil, err
	}

	providerMap := lo.KeyBy(providers, func(p models.Provider) uint { return p.ID })

	weightItems := make(map[uint]int)
	priorityItems := make(map[uint]int)
	for _, mp := range modelWithProviders {
		if _, ok := providerMap[mp.ProviderID]; !ok {
			continue
		}
		weightItems[mp.ID] = mp.Weight
		priorityItems[mp.ID] = mp.Priority
	}

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

// getStrictCapabilityMatch 获取严格能力匹配设置
func getStrictCapabilityMatch(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}
