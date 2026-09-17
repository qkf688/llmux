package healthcheck

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
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

// doCheck 以「诊断口径」打一次真实 completion 判定关联健康：
// 主协议端点 → 按 ID ASC 遍历分组 → 组内首个可用凭据，任一通过即健康。
//
// 为什么必须走 channel 装配而非 providers.New(Type, Provider.Config, Proxy)：S6 起
// Provider.Config 不再承载 api_key（密钥只在 credentials 表、运行时由 BuildConfig 注入
// 明文），直用 Config 会让号池/内联凭据的供应商恒被上游判 401，再经 handleCheckResult
// 把完全可用的关联自动禁用（本函数即 #21 修复点，与模型同步/上游浏览/凭据探活同源）。
//
// 为什么逐组尝试、任一通过即成功：本处失败的代价是「禁用关联」，而「首个分组的 key
// 坏了」并不等于这条关联不可用（生产路由本来就会故障转移到其余分组）。成功即短路，
// 失败请求不产生 token 费用，额外代价可接受。
//
// 为什么凭据口径（NewClientForGroup 内部的 FirstUsableCredential）而非 DefaultSelector().Select：
// ① 检测要可复现（配置没动则每轮同一条 key，失败趋势才有意义）；② 不得推进 chat 链路的
// 轮询指针（一轮巡检就把真实流量的 key 分配挪一格）；③ 分组白名单不参与过滤
// （GroupCandidates 的口径），否则未同步/白名单未含该模型的组会被误判成不可用。
func (h *HealthChecker) doCheck(ctx context.Context, provider *models.Provider, mp *models.ModelWithProvider) error {
	snapshot, err := channel.NewAssembler(repos()).Assemble(ctx, *provider)
	if err != nil {
		return err
	}
	endpoint, ok := channel.SelectPrimaryEndpoint(provider.Type, snapshot.Endpoints)
	if !ok {
		return fmt.Errorf("no enabled %s endpoint for provider %q", consts.ProtocolOfType(provider.Type), provider.Name)
	}
	groups := channel.GroupCandidates(snapshot)
	if len(groups) == 0 {
		return fmt.Errorf("provider %q has no key groups", provider.Name)
	}

	testBody := chooseTestBody(provider.Type)
	now := time.Now()

	// 逐组失败原因都留下：health_check_logs.error 是排查入口，只报「失败」等于丢掉线索。
	failures := make([]string, 0, len(groups))
	for _, group := range groups {
		client, err := channel.NewClientForGroup(snapshot, group, endpoint, now)
		if err != nil {
			failures = append(failures, fmt.Sprintf("group %q: %s", group.Name, err))
			continue
		}
		// headers 每次重建：BuildReq 会就地写认证头，跨组复用同一 map 会互相污染。
		if err := requestUpstream(ctx, client, buildRequestHeaders(mp), mp.ProviderModel, testBody); err != nil {
			failures = append(failures, fmt.Sprintf("group %q: %s", group.Name, err))
			continue
		}
		return nil
	}

	return fmt.Errorf("health check failed for provider %q: %s", provider.Name, strings.Join(failures, "; "))
}

// requestUpstream 发一次检测请求并判定状态码：未通过（口径见 common.IsPassingStatus，
// 仅 200/201）一律带原文回传（HealthCheckError）。
func requestUpstream(ctx context.Context, client providers.Provider, headers http.Header, model string, body []byte) error {
	req, err := client.BuildReq(ctx, headers, model, body)
	if err != nil {
		return err
	}

	httpClient := providers.GetClientWithProxy(30*time.Second, client.GetProxy())
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if !common.IsPassingStatus(res.StatusCode) {
		raw, _ := common.ReadCappedBody(res.Body)
		return &HealthCheckError{StatusCode: res.StatusCode, Body: string(raw)}
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

	bgtask.Go(EnforceHealthCheckLogRetention)
	return nil
}

func chooseTestBody(providerType string) []byte {
	// 未知 type 回退 OpenAI body（与 testapi 对未知 type 报错不同，必须保留）。
	if m, ok := providers.MetadataOf(providerType); ok && len(m.HealthCheckBody) > 0 {
		return m.HealthCheckBody
	}
	if m, ok := providers.MetadataOf(providers.TypeOpenAI); ok && len(m.HealthCheckBody) > 0 {
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
