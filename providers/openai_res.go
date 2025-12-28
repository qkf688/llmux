package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tidwall/sjson"
)

// openai responses api
type OpenAIRes struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	CustomModels []string `json:"custom_models"`
	Proxy        string   `json:"proxy"`
}

func (o *OpenAIRes) BuildReq(ctx context.Context, header http.Header, model string, rawBody []byte) (*http.Request, error) {
	body, err := sjson.SetBytes(rawBody, "model", model)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/responses", o.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if header != nil {
		req.Header = header
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.APIKey))

	return req, nil
}

func (o *OpenAIRes) Models(ctx context.Context) ([]Model, error) {
	if len(o.CustomModels) > 0 {
		return buildCustomModels(o.CustomModels), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", o.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.APIKey))

	// 使用带代理的客户端
	client := GetClientWithProxy(30*time.Second, o.Proxy)
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

	var modelList ModelList
	if err := json.NewDecoder(res.Body).Decode(&modelList); err != nil {
		return nil, err
	}
	return modelList.Data, nil
}

func (o *OpenAIRes) GetProxy() string {
	return o.Proxy
}
