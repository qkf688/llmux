package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/sjson"
)

type Anthropic struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	Version      string   `json:"version"`
	Beta         string   `json:"beta"`
	CustomModels []string `json:"custom_models"`
	Proxy        string   `json:"proxy"`
	AuthType     string   `json:"auth_type"` // x-api-key 或 bearer
}

func init() {
	Register(consts.StyleAnthropic, func(config, proxy string) (Provider, error) {
		var anthropic Anthropic
		if err := json.Unmarshal([]byte(config), &anthropic); err != nil {
			return nil, errors.New("invalid anthropic config")
		}
		if proxy != "" {
			anthropic.Proxy = proxy
		}
		return &anthropic, nil
	})
	RegisterMetadata(Metadata{
		Type:            consts.StyleAnthropic,
		ConfigTemplate:  configTemplateAnthropic,
		TestBody:        []byte(testBodyAnthropic),
		StructuredBody:  []byte(structuredBodyAnthropic),
		HealthCheckBody: []byte(healthCheckBodyAnthropic),
	})
}

func (a *Anthropic) BuildReq(ctx context.Context, header http.Header, model string, rawBody []byte) (*http.Request, error) {
	body, err := sjson.SetBytes(rawBody, "model", model)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/messages", a.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if header != nil {
		req.Header = header
	}
	req.Header.Set("content-type", "application/json")

	// 根据 AuthType 设置认证头
	if a.AuthType == "bearer" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.APIKey))
	} else {
		// 默认使用 x-api-key（Anthropic 官方）
		req.Header.Set("x-api-key", a.APIKey)
	}

	req.Header.Set("anthropic-version", a.Version)
	setAnthropicBeta(req.Header, a.Beta)
	return req, nil
}

// setAnthropicBeta 按 provider 配置写 anthropic-beta 头，Beta 为空时删键而非写空值。
//
// 为什么必须 Del 而不是 Set("")：BuildReq 的 header 参数在关联开了 withHeader 时是客户端
// 请求头的 Clone（chatcore.BuildHeaders），里面可能已带客户端自己的 anthropic-beta。
// provider 未配 Beta 的语义是「本供应商不启用任何 beta 特性」——运维配置权威，不能让客户端
// 头绕过它，故要清掉残留键；而 Set("") 发出的是一个空值头，语义不等于「无此头」。
func setAnthropicBeta(header http.Header, beta string) {
	if beta == "" {
		header.Del("anthropic-beta")
		return
	}
	header.Set("anthropic-beta", beta)
}

// HasBetaFeature 判断本供应商配置的 anthropic-beta 是否包含指定特性（实现 BetaFeatureCapable）。
//
// 判据只看 provider 配置的 Beta 字段，不看客户端请求头——setAnthropicBeta 已保证客户端
// 自带的 anthropic-beta 进不到上游（未配则 Del、配了则覆盖），故运维配置是唯一事实来源。
// Beta 按官方格式是逗号分隔的多特性列表（如 "interleaved-thinking-2025-05-14,output-128k-2025-02-19"），
// 逐项 trim 后比较；官方特性名恒为小写，但运维手填大小写不可控，故用 EqualFold 宽松匹配。
func (a *Anthropic) HasBetaFeature(name string) bool {
	if a.Beta == "" || name == "" {
		return false
	}
	for _, item := range strings.Split(a.Beta, ",") {
		if strings.EqualFold(strings.TrimSpace(item), name) {
			return true
		}
	}
	return false
}

type AnthropicModelsResponse struct {
	Data    []AnthropicModel `json:"data"`
	FirstID string           `json:"first_id"`
	HasMore bool             `json:"has_more"`
	LastID  string           `json:"last_id"`
}

type AnthropicModel struct {
	CreatedAt   time.Time `json:"created_at"`
	DisplayName string    `json:"display_name"`
	ID          string    `json:"id"`
	Type        string    `json:"type"`
}

func (a *Anthropic) Models(ctx context.Context) ([]Model, error) {
	if len(a.CustomModels) > 0 {
		return buildCustomModels(a.CustomModels), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", a.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")

	// 根据 AuthType 设置认证头
	if a.AuthType == "bearer" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.APIKey))
	} else {
		// 默认使用 x-api-key（Anthropic 官方）
		req.Header.Set("x-api-key", a.APIKey)
	}

	req.Header.Set("anthropic-version", a.Version)
	setAnthropicBeta(req.Header, a.Beta)

	// 使用带代理的客户端
	client := GetClientWithProxy(30*time.Second, a.Proxy)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// 读取响应体以获取详细错误信息
		bodyBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return nil, fmt.Errorf("status code: %d, failed to read response body: %v", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("status code: %d, response: %s", res.StatusCode, string(bodyBytes))
	}
	var anthropicModels AnthropicModelsResponse
	if err := json.NewDecoder(res.Body).Decode(&anthropicModels); err != nil {
		return nil, err
	}

	var modelList ModelList
	for _, model := range anthropicModels.Data {
		modelList.Data = append(modelList.Data, Model{
			ID:      model.ID,
			Created: model.CreatedAt.Unix(),
		})
	}
	return modelList.Data, nil
}

func (a *Anthropic) GetProxy() string {
	return a.Proxy
}
