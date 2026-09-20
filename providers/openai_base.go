package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/tidwall/sjson"
)

// openaiBase 是 OpenAI 与 OpenAIRes 供应商的公共基类。
// 两者仅 endpoint 路径不同（/chat/completions vs /responses），
// 其余 BuildReq/Models/GetProxy 逻辑完全一致，通过嵌入复用。
type openaiBase struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	CustomModels []string `json:"custom_models"`
	Proxy        string   `json:"proxy"`
	// AuthType 认证方式：bearer（默认，空值=现状）| x-api-key（Stage 4b，providers/auth.go
	// 单源执法：只发选定方式，对侧凭据头在构造点删除，自定义头配不动）。
	// config 键独立为 openai_auth_type（审查 P0 裁决）：auth_type 键归 anthropic 独占——
	// 混合协议供应商的同一份 config 会喂给两家 adapter，旧行里 auth_type=x-api-key 是
	// anthropic 方言，若 openai 也读它就会在无人保存时把 openai 端点静默翻转成 x-api-key。
	// 按协议拆键后各 adapter 只认自己的键，openai 系存量行永远回退 bearer。
	AuthType     string `json:"openai_auth_type"`
	endpointPath string // 注入差异点：chat/completions 或 responses
}

// OpenAICompatBaseURL / OpenAICompatAPIKey / OpenAICompatAuthHeaders 实现 OpenAICompat 能力接口（OCP-9）。
func (o *openaiBase) OpenAICompatBaseURL() string { return o.BaseURL }
func (o *openaiBase) OpenAICompatAPIKey() string  { return o.APIKey }

// OpenAICompatAuthHeaders 走 applyAuthHeader 单源按方言写好凭据头（Stage 15）：
// 诊断路径（ReAct 测试 SDK 直发）拿这里的头直接用，方言判断不外溢到 handler 层。
func (o *openaiBase) OpenAICompatAuthHeaders() http.Header {
	h := http.Header{}
	applyAuthHeader(h, o.AuthType, AuthTypeBearer, o.APIKey)
	return h
}

func (o *openaiBase) BuildReq(ctx context.Context, header http.Header, model string, rawBody []byte) (*http.Request, error) {
	body, err := sjson.SetBytes(rawBody, "model", model)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/%s", o.BaseURL, o.endpointPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if header != nil {
		req.Header = header
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuthHeader(req.Header, o.AuthType, AuthTypeBearer, o.APIKey)

	return req, nil
}

func (o *openaiBase) Models(ctx context.Context) ([]Model, error) {
	if len(o.CustomModels) > 0 {
		return buildCustomModels(o.CustomModels), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", o.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	// 模型列表旁路同样走 auth_type 单源（与 BuildReq 同一执法点，Stage 4b）。
	applyAuthHeader(req.Header, o.AuthType, AuthTypeBearer, o.APIKey)

	// 使用带代理的客户端
	client := GetClientWithProxy(30*time.Second, o.Proxy)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// 通过口径单源（Stage 5，common.IsPassingStatus：仅 200/201）：回 201 的兼容网关不再被判失败。
	if !common.IsPassingStatus(res.StatusCode) {
		// 读取响应体以获取详细错误信息（上限内，防超大错误页整块进内存）
		bodyBytes, readErr := common.ReadCappedBody(res.Body)
		if readErr != nil {
			return nil, fmt.Errorf("status code: %d, failed to read response body: %v", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("status code: %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	var modelList ModelList
	if err := json.NewDecoder(res.Body).Decode(&modelList); err != nil {
		// 状态码必须留在错误里：通过口径带上 201 后，「状态码通过但内容不是模型列表」
		// 的可达面变宽，只回 `EOF` 会让模型同步日志查不出是哪一段失败。
		return nil, fmt.Errorf("decode model list (HTTP %d): %w", res.StatusCode, err)
	}
	return modelList.Data, nil
}

func (o *openaiBase) GetProxy() string {
	return o.Proxy
}
