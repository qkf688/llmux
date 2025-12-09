package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/atopos31/llmio/consts"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"gorm.io/gorm"
)

const (
	testOpenAIBody = `{
        "model": "gpt-4.1",
        "messages": [
            {
                "role": "user",
                "content": "Write a one-sentence bedtime story about a unicorn."
            }
        ]
    }`

	testOpenAIResBody = `{
        "model": "gpt-5-nano",
        "input": "Write a one-sentence bedtime story about a unicorn."
    }`

	testAnthropicBody = `{
    	"model": "claude-sonnet-4-5",
    	"max_tokens": 1000,
    	"messages": [
      		{
        		"role": "user", 
        		"content": "Write a one-sentence bedtime story about a unicorn."
      		}
    	]
 	}`
)

// HealthChecker 健康检测服务
type HealthChecker struct {
	ctx        context.Context
	cancel     context.CancelFunc
	ticker     *time.Ticker
	mu         sync.RWMutex
	running    bool
	interval   time.Duration
	httpClient *http.Client
}

var (
	healthChecker     *HealthChecker
	healthCheckerOnce sync.Once
)

// GetHealthChecker 获取健康检测单例
func GetHealthChecker() *HealthChecker {
	healthCheckerOnce.Do(func() {
		healthChecker = &HealthChecker{
			httpClient: &http.Client{Timeout: 30 * time.Second},
		}
	})
	return healthChecker
}

// Start 启动健康检测服务
func (h *HealthChecker) Start(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		slog.Info("health checker already running")
		return
	}

	// 检查是否启用健康检测
	enabled := h.isEnabled(ctx)
	if !enabled {
		slog.Info("health check is disabled")
		return
	}

	// 获取检测间隔
	interval := h.getInterval(ctx)
	h.interval = interval

	h.ctx, h.cancel = context.WithCancel(ctx)
	h.ticker = time.NewTicker(interval)
	h.running = true

	go h.run()
	slog.Info("health checker started", "interval", interval)
}

// Stop 停止健康检测服务
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	if h.cancel != nil {
		h.cancel()
	}
	if h.ticker != nil {
		h.ticker.Stop()
	}
	h.running = false
	slog.Info("health checker stopped")
}

// Restart 重启健康检测服务（配置变更时调用）
func (h *HealthChecker) Restart(ctx context.Context) {
	h.Stop()
	h.Start(ctx)
}

// IsRunning 检查是否正在运行
func (h *HealthChecker) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// run 运行健康检测循环
func (h *HealthChecker) run() {
	// 立即执行一次检测
	h.checkAll()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.ticker.C:
			// 检查是否仍然启用
			if !h.isEnabled(h.ctx) {
				slog.Info("health check disabled, stopping checker")
				h.Stop()
				return
			}
			// 检查间隔是否变更
			newInterval := h.getInterval(h.ctx)
			if newInterval != h.interval {
				h.mu.Lock()
				h.interval = newInterval
				h.ticker.Reset(newInterval)
				h.mu.Unlock()
				slog.Info("health check interval updated", "interval", newInterval)
			}
			h.checkAll()
		}
	}
}

// checkAll 检查所有启用的模型提供商
func (h *HealthChecker) checkAll() {
	ctx := context.Background()

	// 获取所有模型提供商关联
	modelProviders, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		slog.Error("failed to get model providers for health check", "error", err)
		return
	}

	slog.Info("starting health check", "count", len(modelProviders))

	for _, mp := range modelProviders {
		h.checkOne(ctx, &mp)
	}

	slog.Info("health check completed")
}

// checkOne 检查单个模型提供商
func (h *HealthChecker) checkOne(ctx context.Context, mp *models.ModelWithProvider) {
	start := time.Now()

	// 获取提供商信息
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", mp.ProviderID).First(ctx)
	if err != nil {
		slog.Error("failed to get provider for health check", "provider_id", mp.ProviderID, "error", err)
		return
	}

	// 获取模型信息
	model, err := gorm.G[models.Model](models.DB).Where("id = ?", mp.ModelID).First(ctx)
	if err != nil {
		slog.Error("failed to get model for health check", "model_id", mp.ModelID, "error", err)
		return
	}

	// 执行检测
	checkErr := h.doCheck(ctx, &provider, mp)
	responseTime := time.Since(start).Milliseconds()

	// 记录日志
	log := models.HealthCheckLog{
		ModelProviderID: mp.ID,
		ModelName:       model.Name,
		ProviderName:    provider.Name,
		ProviderModel:   mp.ProviderModel,
		ResponseTime:    responseTime,
		CheckedAt:       time.Now(),
	}

	if checkErr != nil {
		log.Status = "error"
		log.Error = checkErr.Error()
		slog.Warn("health check failed", "model", model.Name, "provider", provider.Name, "error", checkErr)
	} else {
		log.Status = "success"
		slog.Info("health check passed", "model", model.Name, "provider", provider.Name, "response_time", responseTime)
	}

	// 保存日志
	if err := gorm.G[models.HealthCheckLog](models.DB).Create(ctx, &log); err != nil {
		slog.Error("failed to save health check log", "error", err)
	}

	go EnforceHealthCheckLogRetention(context.Background())

	// 处理检测结果
	h.handleCheckResult(ctx, mp, checkErr == nil)
}

// doCheck 执行实际的检测请求
func (h *HealthChecker) doCheck(ctx context.Context, provider *models.Provider, mp *models.ModelWithProvider) error {
	// 创建提供商实例
	providerInstance, err := providers.New(provider.Type, provider.Config, provider.Proxy)
	if err != nil {
		return err
	}

	// 根据类型选择测试请求体
	var testBody []byte
	switch provider.Type {
	case consts.StyleOpenAI:
		testBody = []byte(testOpenAIBody)
	case consts.StyleAnthropic:
		testBody = []byte(testAnthropicBody)
	case consts.StyleOpenAIRes:
		testBody = []byte(testOpenAIResBody)
	default:
		testBody = []byte(testOpenAIBody)
	}

	// 构建请求
	header := http.Header{}
	if mp.WithHeader != nil && *mp.WithHeader {
		for key, value := range mp.CustomerHeaders {
			header.Set(key, value)
		}
	}

	req, err := providerInstance.BuildReq(ctx, header, mp.ProviderModel, testBody)
	if err != nil {
		return err
	}

	// 发送请求，优先使用提供商级别代理
	client := providers.GetClientWithProxy(30*time.Second, providerInstance.GetProxy())
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return &HealthCheckError{StatusCode: res.StatusCode, Body: string(body)}
	}

	return nil
}

// HealthCheckError 健康检测错误
type HealthCheckError struct {
	StatusCode int
	Body       string
}

func (e *HealthCheckError) Error() string {
	return "health check failed with status " + strconv.Itoa(e.StatusCode) + ": " + e.Body
}

// handleCheckResult 处理检测结果
func (h *HealthChecker) handleCheckResult(ctx context.Context, mp *models.ModelWithProvider, success bool) {
	failureThreshold := h.getFailureThreshold(ctx)
	autoEnable := h.getAutoEnable(ctx)

	if success {
		// 检测成功
		if autoEnable && (mp.Status == nil || !*mp.Status) {
			// 自动启用
			trueVal := true
			if _, err := gorm.G[models.ModelWithProvider](models.DB).
				Where("id = ?", mp.ID).
				Updates(ctx, models.ModelWithProvider{Status: &trueVal}); err != nil {
				slog.Error("failed to enable model provider after health check success", "id", mp.ID, "error", err)
			} else {
				slog.Info("model provider auto-enabled after health check success", "id", mp.ID)
			}
		}
	} else {
		// 检测失败，检查连续失败次数
		failCount, err := h.getConsecutiveFailures(ctx, mp.ID)
		if err != nil {
			slog.Error("failed to get consecutive failures", "id", mp.ID, "error", err)
			return
		}

		if failCount >= failureThreshold && (mp.Status == nil || *mp.Status) {
			// 超过阈值，自动禁用
			falseVal := false
			if _, err := gorm.G[models.ModelWithProvider](models.DB).
				Where("id = ?", mp.ID).
				Updates(ctx, models.ModelWithProvider{Status: &falseVal}); err != nil {
				slog.Error("failed to disable model provider after health check failures", "id", mp.ID, "error", err)
			} else {
				slog.Warn("model provider auto-disabled after health check failures", "id", mp.ID, "fail_count", failCount)
			}
		}
	}
}

// getConsecutiveFailures 获取连续失败次数
func (h *HealthChecker) getConsecutiveFailures(ctx context.Context, mpID uint) (int, error) {
	// 获取最近的检测日志，按时间倒序
	logs, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("model_provider_id = ?", mpID).
		Order("checked_at DESC").
		Limit(10).
		Find(ctx)
	if err != nil {
		return 0, err
	}

	// 计算连续失败次数
	count := 0
	for _, log := range logs {
		if log.Status == "error" {
			count++
		} else {
			break
		}
	}

	return count, nil
}

// isEnabled 检查健康检测是否启用
func (h *HealthChecker) isEnabled(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckEnabled).
		First(ctx)
	if err != nil {
		return false
	}
	return setting.Value == "true"
}

// getInterval 获取检测间隔
func (h *HealthChecker) getInterval(ctx context.Context) time.Duration {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckInterval).
		First(ctx)
	if err != nil {
		return 60 * time.Minute // 默认60分钟
	}
	minutes, err := strconv.Atoi(setting.Value)
	if err != nil || minutes < 1 {
		return 60 * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

// getFailureThreshold 获取失败次数阈值
func (h *HealthChecker) getFailureThreshold(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureThreshold).
		First(ctx)
	if err != nil {
		return 3 // 默认3次
	}
	threshold, err := strconv.Atoi(setting.Value)
	if err != nil || threshold < 1 {
		return 3
	}
	return threshold
}

// getAutoEnable 获取是否自动启用
func (h *HealthChecker) getAutoEnable(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckAutoEnable).
		First(ctx)
	if err != nil {
		return false
	}
	return setting.Value == "true"
}

// getLogRetentionCount 获取健康检测日志保留条数
func (h *HealthChecker) getLogRetentionCount(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckLogRetentionCount).
		First(ctx)
	if err != nil {
		return 0
	}
	retention, err := strconv.Atoi(setting.Value)
	if err != nil || retention < 0 {
		return 0
	}
	return retention
}

// CheckSingle 手动检测单个模型提供商
func (h *HealthChecker) CheckSingle(ctx context.Context, mpID uint) (*models.HealthCheckLog, error) {
	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", mpID).First(ctx)
	if err != nil {
		return nil, err
	}

	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", mp.ProviderID).First(ctx)
	if err != nil {
		return nil, err
	}

	model, err := gorm.G[models.Model](models.DB).Where("id = ?", mp.ModelID).First(ctx)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	checkErr := h.doCheck(ctx, &provider, &mp)
	responseTime := time.Since(start).Milliseconds()

	log := models.HealthCheckLog{
		ModelProviderID: mp.ID,
		ModelName:       model.Name,
		ProviderName:    provider.Name,
		ProviderModel:   mp.ProviderModel,
		ResponseTime:    responseTime,
		CheckedAt:       time.Now(),
	}

	if checkErr != nil {
		log.Status = "error"
		log.Error = checkErr.Error()
	} else {
		log.Status = "success"
	}

	// 保存日志
	if err := gorm.G[models.HealthCheckLog](models.DB).Create(ctx, &log); err != nil {
		return nil, err
	}

	go EnforceHealthCheckLogRetention(context.Background())

	// 处理检测结果
	h.handleCheckResult(ctx, &mp, checkErr == nil)

	return &log, nil
}

// GetHealthCheckSettings 获取健康检测设置
func GetHealthCheckSettings(ctx context.Context) (enabled bool, interval int, failureThreshold int, autoEnable bool, logRetentionCount int) {
	checker := GetHealthChecker()

	enabled = checker.isEnabled(ctx)

	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckInterval).
		First(ctx)
	if err == nil {
		interval, _ = strconv.Atoi(setting.Value)
	}
	if interval < 1 {
		interval = 60
	}

	failureThreshold = checker.getFailureThreshold(ctx)
	autoEnable = checker.getAutoEnable(ctx)
	logRetentionCount = checker.getLogRetentionCount(ctx)

	return
}

// HealthCheckSettingsJSON 健康检测设置 JSON 结构
type HealthCheckSettingsJSON struct {
	Enabled           bool `json:"enabled"`
	Interval          int  `json:"interval"`
	FailureThreshold  int  `json:"failure_threshold"`
	AutoEnable        bool `json:"auto_enable"`
	LogRetentionCount int  `json:"log_retention_count"`
}

// MarshalJSON 序列化健康检测设置
func (s HealthCheckSettingsJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Enabled           bool `json:"enabled"`
		Interval          int  `json:"interval"`
		FailureThreshold  int  `json:"failure_threshold"`
		AutoEnable        bool `json:"auto_enable"`
		LogRetentionCount int  `json:"log_retention_count"`
	}{
		Enabled:           s.Enabled,
		Interval:          s.Interval,
		FailureThreshold:  s.FailureThreshold,
		AutoEnable:        s.AutoEnable,
		LogRetentionCount: s.LogRetentionCount,
	})
}

// EnforceHealthCheckLogRetention 清理超出保留条数的健康检测日志
func EnforceHealthCheckLogRetention(ctx context.Context) {
	retention := GetHealthChecker().getLogRetentionCount(ctx)
	if retention <= 0 {
		return
	}
	cleanupHealthCheckLogs(ctx, retention)
}

func cleanupHealthCheckLogs(ctx context.Context, retentionCount int) {
	var total int64
	if err := models.DB.WithContext(ctx).Model(&models.HealthCheckLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count health check logs for cleanup", "error", err)
		return
	}

	if int(total) <= retentionCount {
		return
	}

	deleteCount := int(total) - retentionCount
	var ids []uint
	if err := models.DB.WithContext(ctx).
		Model(&models.HealthCheckLog{}).
		Order("id ASC").
		Limit(deleteCount).
		Pluck("id", &ids).Error; err != nil {
		slog.Error("failed to find health check logs to delete", "error", err)
		return
	}

	if len(ids) == 0 {
		return
	}

	if _, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("id IN ?", ids).
		Delete(ctx); err != nil {
		slog.Error("failed to delete health check logs", "error", err)
		return
	}

	slog.Info("cleaned up excess health check logs", "deleted", len(ids), "retention", retentionCount)
}
