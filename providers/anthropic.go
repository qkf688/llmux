package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/sjson"
)

// TypeAnthropic 是本 provider 在 DB Provider.Type 里的取值，也是 Register / RegisterMetadata 的键。
const TypeAnthropic = "anthropic"

type Anthropic struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	Version      string   `json:"version"`
	Beta         string   `json:"beta"`
	CustomModels []string `json:"custom_models"`
	Proxy        string   `json:"proxy"`
	AuthType     string   `json:"auth_type"` // 认证方式 x-api-key（默认）| bearer，providers/auth.go 唯一执法点（4b）。本键归 anthropic 独占；openai 系用独立键 openai_auth_type（混合协议供应商按协议拆方言，审查 P0）
}

func init() {
	Register(TypeAnthropic, func(config, proxy string) (Provider, error) {
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
		Type:            TypeAnthropic,
		WireFormat:      consts.FormatAnthropic,
		ConfigTemplate:  configTemplateAnthropic,
		TestBody:        []byte(testBodyAnthropic),
		StructuredBody:  []byte(structuredBodyAnthropic),
		ThinkingBody:    []byte(thinkingBodyAnthropic),
		VisionBody:      []byte(visionBodyAnthropic),
		HealthCheckBody: []byte(healthCheckBodyAnthropic),
		ProbeBody:       []byte(probeBodyAnthropic),
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

	// 认证头以 auth_type 为唯一权威（providers/auth.go 单源执法）：x-api-key 方式删除
	// 任何配进来的 Authorization，bearer 方式删除 x-api-key——两个凭据头永不并发送达
	// 上游（Stage 4b；上游对双头的裁决是实现定义行为，网关不赌）。
	applyAuthHeader(req.Header, a.AuthType, AuthTypeXAPIKey, a.APIKey)

	req.Header.Set("anthropic-version", a.Version)
	setAnthropicBeta(req.Header, a.Beta)
	return req, nil
}

// setAnthropicBeta 按 provider 配置写 anthropic-beta 头，Beta 为空时删键而非写空值。
//
// 为什么必须 Del 而不是 Set("")：BuildReq 的 header 参数在关联开了 withHeader 时含客户端
// 头的白名单透传（chatcore.BuildHeaders → common 判据放行 Anthropic-Beta），里面可能已带
// 客户端自己的 anthropic-beta。
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

	// 模型列表旁路同样走 auth_type 单源（与 BuildReq 一个执法点，防「对话用一种、
	// 拉模型用另一种」的分裂）。
	applyAuthHeader(req.Header, a.AuthType, AuthTypeXAPIKey, a.APIKey)

	req.Header.Set("anthropic-version", a.Version)
	setAnthropicBeta(req.Header, a.Beta)

	// 使用带代理的客户端
	client := GetClientWithProxy(30*time.Second, a.Proxy)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// 通过口径单源（Stage 5）：2xx 即成功，回 201 的兼容网关不再被判失败。
	if !common.IsPassingStatus(res.StatusCode) {
		// 读取响应体以获取详细错误信息（上限内，防超大错误页整块进内存）
		bodyBytes, readErr := common.ReadCappedBody(res.Body)
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
