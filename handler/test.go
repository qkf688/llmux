package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/consts"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/atopos31/nsxno/react"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

const (
	testOpenAI = `{
        "model": "gpt-4.1",
        "messages": [
            {
                "role": "user",
                "content": "Write a one-sentence bedtime story about a unicorn."
            }
        ]
    }`

	testOpenAIRes = `{
        "model": "gpt-5-nano",
        "input": "Write a one-sentence bedtime story about a unicorn."
    }`

	testAnthropic = `{
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

type ProviderModelTestRequest struct {
	Model string `json:"model"`
}

func ProviderTestHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		common.BadRequest(c, "Invalid ID format")
		return
	}
	ctx := c.Request.Context()

	chatModel, err := FindChatModel(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "ModelWithProvider not found")
			return
		}
		common.InternalServerError(c, "Database error")
		return
	}

	// Create the provider instance
	providerInstance, err := providers.New(chatModel.Type, chatModel.Config, chatModel.Proxy)
	if err != nil {
		common.BadRequest(c, "Failed to create provider: "+err.Error())
		return
	}

	// Test connectivity by fetching models
	proxyURL := providerInstance.GetProxy()
	slog.Info("Testing provider", "proxy", proxyURL, "provider", chatModel.Name)
	client := providers.GetClientWithProxy(time.Second*time.Duration(60), proxyURL)
	var testBody []byte
	switch chatModel.Type {
	case consts.StyleOpenAI:
		testBody = []byte(testOpenAI)
	case consts.StyleAnthropic:
		testBody = []byte(testAnthropic)
	case consts.StyleOpenAIRes:
		testBody = []byte(testOpenAIRes)
	default:
		common.BadRequest(c, "Invalid provider type")
		return
	}
	header := buildTestHeaders(c.Request.Header, chatModel.WithHeader, chatModel.CustomerHeaders)
	req, err := providerInstance.BuildReq(ctx, header, chatModel.Model, []byte(testBody))
	if err != nil {
		common.ErrorWithHttpStatus(c, http.StatusOK, 502, buildDetailedError("network", "构建请求失败", err.Error(), map[string]string{
			"provider": chatModel.Name,
			"model":    chatModel.Model,
		}))
		return
	}
	res, err := client.Do(req)
	if err != nil {
		common.ErrorWithHttpStatus(c, http.StatusOK, 502, buildDetailedError("network", "连接提供商失败", err.Error(), map[string]string{
			"provider": chatModel.Name,
			"model":    chatModel.Model,
			"proxy":    proxyURL,
		}))
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// 读取响应体以获取更详细的错误信息
		bodyBytes, _ := io.ReadAll(res.Body)
		bodyStr := string(bodyBytes)

		// 尝试解析JSON错误响应
		var errorDetail string
		var jsonErr map[string]interface{}
		if json.Unmarshal(bodyBytes, &jsonErr) == nil {
			if errMsg, ok := jsonErr["error"].(map[string]interface{}); ok {
				if message, ok := errMsg["message"].(string); ok {
					errorDetail = message
				} else if msg, ok := errMsg["error"].(string); ok {
					errorDetail = msg
				}
			}
		}
		if errorDetail == "" {
			errorDetail = bodyStr
		}

		common.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, buildDetailedError(getErrorTypeFromStatus(res.StatusCode), "提供商返回错误", errorDetail, map[string]string{
			"provider":    chatModel.Name,
			"model":       chatModel.Model,
			"status_code": strconv.Itoa(res.StatusCode),
		}))
		return
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		common.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, buildDetailedError("network", "读取响应失败", err.Error(), map[string]string{
			"provider": chatModel.Name,
			"model":    chatModel.Model,
		}))
		return
	}

	common.SuccessWithMessage(c, string(content), nil)
}

func ProviderModelTestHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		common.BadRequest(c, "Invalid ID format")
		return
	}
	var req ProviderModelTestRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Model) == "" {
		common.BadRequest(c, "Invalid model name")
		return
	}
	ctx := c.Request.Context()

	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Provider not found")
			return
		}
		common.InternalServerError(c, "Database error")
		return
	}

	providerInstance, err := providers.New(provider.Type, provider.Config, provider.Proxy)
	if err != nil {
		common.BadRequest(c, "Failed to create provider: "+err.Error())
		return
	}

	proxyURL := providerInstance.GetProxy()
	slog.Info("Testing provider model", "proxy", proxyURL, "provider", provider.Name, "model", req.Model)
	client := providers.GetClientWithProxy(time.Second*time.Duration(60), proxyURL)
	var testBody []byte
	switch provider.Type {
	case consts.StyleOpenAI:
		testBody = []byte(testOpenAI)
	case consts.StyleAnthropic:
		testBody = []byte(testAnthropic)
	case consts.StyleOpenAIRes:
		testBody = []byte(testOpenAIRes)
	default:
		common.BadRequest(c, "Invalid provider type")
		return
	}
	header := buildTestHeaders(c.Request.Header, nil, nil)
	testReq, err := providerInstance.BuildReq(ctx, header, req.Model, []byte(testBody))
	if err != nil {
		common.ErrorWithHttpStatus(c, http.StatusOK, 502, "Failed to connect to provider: "+err.Error())
		return
	}
	res, err := client.Do(testReq)
	if err != nil {
		common.ErrorWithHttpStatus(c, http.StatusOK, 502, "Failed to connect to provider: "+err.Error())
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		common.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, "Provider returned non-200 status code: "+strconv.Itoa(res.StatusCode))
		return
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		common.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, "Failed to read res body: "+err.Error())
		return
	}

	common.SuccessWithMessage(c, string(content), nil)
}

func TestReactHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	chatModel, err := FindChatModel(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "ModelWithProvider not found")
			return
		}
		common.InternalServerError(c, "Database error")
		return
	}

	if chatModel.Type != "openai" {
		c.SSEvent("error", "该测试仅支持 OpenAI 类型")
		return
	}

	var config providers.OpenAI
	if err := json.Unmarshal([]byte(chatModel.Config), &config); err != nil {
		common.ErrorWithHttpStatus(c, http.StatusBadRequest, 400, "Invalid config format")
		return
	}

	client := openai.NewClient(
		option.WithBaseURL(config.BaseURL),
		option.WithAPIKey(config.APIKey),
	)

	agent := react.New(client, 20)
	question := "分两次获取一下南京和北京的天气 每次调用后回复我对应城市的总结信息"
	model := chatModel.Model

	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "get_weather",
			Description: openai.String("Get weather at the given location"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]string{
						"type":        "string",
						"description": "The city name",
					},
				},
				"required": []string{"location"},
			},
		}),
	}
	var checkError error
	var toolCount int
	var nankingCount int
	var pekingCount int

	c.SSEvent("start", fmt.Sprintf("提供商:%s 模型:%s 问题:%s", chatModel.Name, chatModel.Model, question))
	start := time.Now()
	for content, err := range agent.RunStream(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools: tools,
		Model: model,
	}, GetWeather) {
		if err != nil {
			c.SSEvent("error", err.Error())
			break
		}
		var res string
		switch content.Cate {
		case "message":
			if len(content.Chunk.Choices) > 0 {
				res = content.Chunk.Choices[0].Delta.Content
			}
		case "toolcall":
			data, err := json.Marshal(content.ToolCall.Function)
			if err != nil {
				c.SSEvent("error", err.Error())
				break
			}
			res = string(data)
			location := gjson.Get(content.ToolCall.Function.Arguments, "location").String()
			if location == "南京" {
				nankingCount++
			}
			if location == "北京" {
				pekingCount++
			}
			if content.Step == 0 && location != "南京" {
				checkError = errors.New("第一次应选择南京")
			}
			if content.Step == 1 && location != "北京" {
				checkError = errors.New("第二次应选择北京")
			}
			toolCount++
		case "toolres":
			data, err := json.Marshal(content.ToolRes)
			if err != nil {
				c.SSEvent("error", err.Error())
				break
			}
			res = string(data)
		}
		c.SSEvent(content.Cate, res)
		c.Writer.Flush()
	}
	if toolCount != 2 || nankingCount != 1 || pekingCount != 1 {
		checkError = fmt.Errorf("工具调用次数异常: 南京: %d 北京: %d 总计: %d", nankingCount, pekingCount, toolCount)
	}

	if checkError != nil {
		c.SSEvent("error", checkError.Error())
		c.Writer.Flush()
		return
	}
	c.SSEvent("success", fmt.Sprintf("成功通过测试, 耗时: %.2fs", time.Since(start).Seconds()))
}

func GetWeather(ctx context.Context, call openai.ChatCompletionChunkChoiceDeltaToolCallFunction) (*openai.ChatCompletionToolMessageParamContentUnion, error) {
	if call.Name != "get_weather" {
		return nil, fmt.Errorf("invalid tool call name: %s", call.Name)
	}
	location := gjson.Get(call.Arguments, "location")
	var res string
	switch location.String() {
	case "南京":
		res = "南京天气晴转多云，温度 18℃"
	case "北京":
		res = "北京天气大雨转小雨，温度 15℃"
	default:
		res = "暂不支持该地区天气查询"
	}
	return &openai.ChatCompletionToolMessageParamContentUnion{
		OfString: openai.String(res),
	}, nil
}

type ChatModel struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Model           string            `json:"model"`
	Config          string            `json:"config"`
	Proxy           string            `json:"proxy"`
	WithHeader      *bool             `json:"with_header,omitempty"`
	CustomerHeaders map[string]string `json:"customer_headers,omitempty"`
}

func FindChatModel(ctx context.Context, id string) (*ChatModel, error) {
	// Get ModelWithProvider by ID
	modelWithProvider, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		return nil, err
	}

	// Get the Provider
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", modelWithProvider.ProviderID).First(ctx)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		Name:            provider.Name,
		Type:            provider.Type,
		Model:           modelWithProvider.ProviderModel,
		Config:          provider.Config,
		Proxy:           provider.Proxy,
		WithHeader:      modelWithProvider.WithHeader,
		CustomerHeaders: modelWithProvider.CustomerHeaders,
	}, nil
}

func buildTestHeaders(source http.Header, withHeader *bool, customHeaders map[string]string) http.Header {
	header := http.Header{}

	if withHeader != nil && *withHeader {
		header = source.Clone()
	}

	for key, value := range customHeaders {
		header.Set(key, value)
	}

	return header
}

// buildDetailedError 构建详细的错误信息
func buildDetailedError(errorType string, summary string, detail string, context map[string]string) string {
	// 构建错误信息
	var sb strings.Builder
	
	// 错误类型和概要
	sb.WriteString(fmt.Sprintf("[%s] %s\n\n", getErrorTypeLabel(errorType), summary))
	
	// 详细信息
	sb.WriteString(fmt.Sprintf("详细信息: %s\n", detail))
	
	// 上下文信息
	if len(context) > 0 {
		sb.WriteString("\n上下文信息:\n")
		for key, value := range context {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", key, value))
		}
	}
	
	return sb.String()
}

// getErrorTypeFromStatus 根据HTTP状态码返回错误类型
func getErrorTypeFromStatus(statusCode int) string {
	switch {
	case statusCode == 401 || statusCode == 403:
		return "auth"
	case statusCode == 404:
		return "provider"
	case statusCode >= 400 && statusCode < 500:
		return "validation"
	case statusCode >= 500:
		return "provider"
	default:
		return "unknown"
	}
}

// getErrorTypeLabel 获取错误类型的可读标签
func getErrorTypeLabel(errorType string) string {
	labels := map[string]string{
		"network":   "网络错误",
		"auth":      "认证错误",
		"provider":  "提供商错误",
		"timeout":   "超时错误",
		"validation": "验证错误",
		"unknown":   "未知错误",
	}
	if label, ok := labels[errorType]; ok {
		return label
	}
	return "未知错误"
}
