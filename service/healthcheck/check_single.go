package healthcheck

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/atopos31/llmio/consts"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
)

func (h *HealthChecker) checkOne(ctx context.Context, mp *models.ModelWithProvider) {
	h.checkOneWithBatch(ctx, mp, "")
}

func (h *HealthChecker) checkOneWithBatch(ctx context.Context, mp *models.ModelWithProvider, batchID string) {
	start := time.Now()

	provider, model, err := getProviderAndModel(ctx, mp)
	if err != nil {
		return
	}

	checkErr := h.doCheck(ctx, provider, mp)
	responseTime := time.Since(start).Milliseconds()
	logEntry := buildHealthCheckLog(batchID, mp, provider.Name, model.Name, responseTime)

	if checkErr != nil {
		logEntry.Status = "error"
		logEntry.Error = checkErr.Error()
		slog.Warn("health check failed", "model", model.Name, "provider", provider.Name, "error", checkErr, "batch_id", batchID)
	} else {
		logEntry.Status = "success"
		slog.Info("health check passed", "model", model.Name, "provider", provider.Name, "response_time", responseTime, "batch_id", batchID)
	}

	if err := h.saveHealthCheckLog(ctx, &logEntry); err != nil {
		slog.Error("failed to save health check log", "error", err)
	}

	h.handleCheckResult(ctx, mp, provider.Name, checkErr == nil)
}

// CheckSingle 手动检测单个模型提供商。
func (h *HealthChecker) CheckSingle(ctx context.Context, mpID uint) (*models.HealthCheckLog, error) {
	return h.CheckSingleWithBatch(ctx, mpID, "")
}

// CheckSingleWithBatch 手动检测单个模型提供商（支持 batchID）。
func (h *HealthChecker) CheckSingleWithBatch(ctx context.Context, mpID uint, batchID string) (*models.HealthCheckLog, error) {
	mp, err := repos().ModelWithProvider.Get(ctx, mpID)
	if err != nil {
		return nil, err
	}

	provider, model, err := getProviderAndModel(ctx, mp)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	checkErr := h.doCheck(ctx, provider, mp)
	responseTime := time.Since(start).Milliseconds()
	logEntry := buildHealthCheckLog(batchID, mp, provider.Name, model.Name, responseTime)

	if checkErr != nil {
		logEntry.Status = "error"
		logEntry.Error = checkErr.Error()
	} else {
		logEntry.Status = "success"
	}

	if err := h.saveHealthCheckLog(ctx, &logEntry); err != nil {
		return nil, err
	}

	h.handleCheckResult(ctx, mp, provider.Name, checkErr == nil)
	return &logEntry, nil
}

func (h *HealthChecker) doCheck(ctx context.Context, provider *models.Provider, mp *models.ModelWithProvider) error {
	providerInstance, err := providers.New(provider.Type, provider.Config, provider.Proxy)
	if err != nil {
		return err
	}

	testBody := chooseTestBody(provider.Type)
	headers := buildRequestHeaders(mp)

	req, err := providerInstance.BuildReq(ctx, headers, mp.ProviderModel, testBody)
	if err != nil {
		return err
	}

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

func getProviderAndModel(ctx context.Context, mp *models.ModelWithProvider) (*models.Provider, *models.Model, error) {
	provider, err := repos().Provider.Get(ctx, mp.ProviderID)
	if err != nil {
		slog.Error("failed to get provider for health check", "provider_id", mp.ProviderID, "error", err)
		return nil, nil, err
	}

	model, err := repos().Model.Get(ctx, mp.ModelID)
	if err != nil {
		slog.Error("failed to get model for health check", "model_id", mp.ModelID, "error", err)
		return nil, nil, err
	}

	return provider, model, nil
}

func buildHealthCheckLog(batchID string, mp *models.ModelWithProvider, providerName, modelName string, responseTime int64) models.HealthCheckLog {
	return models.HealthCheckLog{
		BatchID:         batchID,
		ModelProviderID: mp.ID,
		ModelName:       modelName,
		ProviderName:    providerName,
		ProviderModel:   mp.ProviderModel,
		ResponseTime:    responseTime,
		CheckedAt:       time.Now(),
	}
}

func (h *HealthChecker) saveHealthCheckLog(ctx context.Context, logEntry *models.HealthCheckLog) error {
	if err := repos().HealthCheckLog.Create(ctx, logEntry); err != nil {
		return err
	}

	go EnforceHealthCheckLogRetention(context.Background())
	return nil
}

func chooseTestBody(providerType string) []byte {
	// 未知 type 回退 OpenAI body（与 testapi 对未知 type 报错不同，必须保留）。
	if m, ok := providers.MetadataOf(providerType); ok && len(m.HealthCheckBody) > 0 {
		return m.HealthCheckBody
	}
	if m, ok := providers.MetadataOf(consts.StyleOpenAI); ok && len(m.HealthCheckBody) > 0 {
		return m.HealthCheckBody
	}
	return nil
}

func buildRequestHeaders(mp *models.ModelWithProvider) http.Header {
	headers := http.Header{}
	if mp.WithHeader == nil || !*mp.WithHeader {
		return headers
	}

	for key, value := range mp.CustomerHeaders {
		headers.Set(key, value)
	}
	return headers
}
