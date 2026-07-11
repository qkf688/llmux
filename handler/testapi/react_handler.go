package testapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/consts"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	anthropicsvc "github.com/atopos31/llmio/service/anthropic"
	"github.com/atopos31/nsxno/react"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

type reactScenario struct {
	Question       string
	ExpectedCities []string
}

var reactScenarios = []reactScenario{
	{
		Question:       "分两次获取一下南京和北京的天气 每次调用后回复我对应城市的总结信息",
		ExpectedCities: []string{"南京", "北京"},
	},
	{
		Question:       "分两次获取一下上海和广州的天气 每次调用后回复我对应城市的总结信息",
		ExpectedCities: []string{"上海", "广州"},
	},
	{
		Question:       "分两次获取一下深圳和杭州的天气 每次调用后回复我对应城市的总结信息",
		ExpectedCities: []string{"深圳", "杭州"},
	},
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

	scenarioIndex, err := selectScenarioIndex(id, c.Query("scenario"), len(reactScenarios))
	if err != nil {
		c.SSEvent("error", err.Error())
		c.Writer.Flush()
		return
	}
	scenario := reactScenarios[scenarioIndex]

	providerInstance, err := providers.New(chatModel.Type, chatModel.Config, chatModel.Proxy)
	if err != nil {
		c.SSEvent("error", "创建提供商失败: "+err.Error())
		c.Writer.Flush()
		return
	}

	proxyURL := providerInstance.GetProxy()
	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)

	header := BuildTestHeaders(c.Request.Header, chatModel.WithHeader, chatModel.CustomerHeaders)

	var checkError error
	toolCities := make([]string, 0)
	finalText := strings.Builder{}

	c.SSEvent("start", fmt.Sprintf("提供商:%s 模型:%s 类型:%s 场景:%d 问题:%s", chatModel.Name, chatModel.Model, chatModel.Type, scenarioIndex, scenario.Question))
	c.Writer.Flush()
	start := time.Now()

	// OCP-9：优先 OpenAICompat 能力接口；否则 anthropic 协议族；禁止 *OpenAI/*OpenAIRes/*Anthropic 类型断言。
	if compat, ok := providerInstance.(providers.OpenAICompat); ok {
		baseURL, apiKey := compat.OpenAICompatBaseURL(), compat.OpenAICompatAPIKey()
		clientOptions := []option.RequestOption{
			option.WithBaseURL(baseURL),
			option.WithHTTPClient(httpClient),
		}
		for key, values := range header {
			for _, value := range values {
				clientOptions = append(clientOptions, option.WithHeaderAdd(key, value))
			}
		}
		clientOptions = append(clientOptions, option.WithAPIKey(apiKey))

		client := openai.NewClient(clientOptions...)
		agent := react.New(client, 20)
		tools := []openai.ChatCompletionToolUnionParam{
			openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name:        "get_weather",
				Description: openai.String("Get weather at the given location"),
				Parameters:  openai.FunctionParameters(buildWeatherToolSchema()),
			}),
		}

		for content, err := range agent.RunStream(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(scenario.Question),
			},
			Tools: tools,
			Model: chatModel.Model,
		}, GetWeather) {
			if err != nil {
				checkError = err
				break
			}

			var res string
			switch content.Cate {
			case "message":
				if len(content.Chunk.Choices) > 0 {
					res = content.Chunk.Choices[0].Delta.Content
					if res != "" {
						finalText.WriteString(res)
					}
				}
			case "toolcall":
				data, err := json.Marshal(content.ToolCall.Function)
				if err != nil {
					checkError = err
					break
				}
				res = string(data)

				location := gjson.Get(content.ToolCall.Function.Arguments, "location").String()
				toolCities = append(toolCities, normalizeCity(location))
			case "toolres":
				data, err := json.Marshal(content.ToolRes)
				if err != nil {
					checkError = err
					break
				}
				res = string(data)
			}

			c.SSEvent(content.Cate, res)
			c.Writer.Flush()
		}
	} else if chatModel.Type == consts.StyleAnthropic {
		var final string
		toolCities, final, checkError = runAnthropicReactTest(ctx, c, httpClient, header, chatModel, providerInstance, scenario)
		finalText.WriteString(final)
	} else {
		c.SSEvent("error", fmt.Sprintf("该测试仅支持 OpenAI 兼容接口（%s/%s）或 %s，当前提供商类型: %s", consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic, chatModel.Type))
		c.Writer.Flush()
		return
	}

	if checkError == nil {
		if err := validateReactToolCalls(toolCities, scenario.ExpectedCities); err != nil {
			checkError = err
		}
	}
	if checkError == nil {
		if err := validateReactFinalResponse(finalText.String(), scenario.ExpectedCities); err != nil {
			checkError = err
		}
	}

	if checkError != nil {
		c.SSEvent("error", checkError.Error())
		c.Writer.Flush()
		return
	}

	c.SSEvent("success", fmt.Sprintf("成功通过测试, 耗时: %.2fs", time.Since(start).Seconds()))
	c.Writer.Flush()
}

func GetWeather(ctx context.Context, call openai.ChatCompletionChunkChoiceDeltaToolCallFunction) (*openai.ChatCompletionToolMessageParamContentUnion, error) {
	if call.Name != "get_weather" {
		return nil, fmt.Errorf("invalid tool call name: %s", call.Name)
	}

	_, res := weatherFromArguments(call.Arguments)

	return &openai.ChatCompletionToolMessageParamContentUnion{
		OfString: openai.String(res),
	}, nil
}

func openAICompatConfig(provider providers.Provider) (baseURL, apiKey string, supported bool) {
	compat, ok := provider.(providers.OpenAICompat)
	if !ok {
		return "", "", false
	}
	return compat.OpenAICompatBaseURL(), compat.OpenAICompatAPIKey(), true
}

func runAnthropicReactTest(ctx context.Context, c *gin.Context, httpClient *http.Client, header http.Header, chatModel *ChatModel, providerInstance providers.Provider, scenario reactScenario) ([]string, string, error) {
	if chatModel.Type != consts.StyleAnthropic {
		return nil, "", fmt.Errorf("invalid provider type for anthropic runner: %s", chatModel.Type)
	}
	// 仅依赖 Provider 接口（BuildReq），不强制 *providers.Anthropic 类型断言。
	if providerInstance == nil {
		return nil, "", fmt.Errorf("invalid provider instance: nil")
	}

	tools := []models.UnifiedTool{
		{
			Type: "function",
			Function: models.UnifiedFunc{
				Name:        "get_weather",
				Description: "Get weather at the given location",
				Parameters:  buildWeatherToolSchema(),
			},
		},
	}

	toolCities := make([]string, 0)
	finalText := strings.Builder{}
	messages := []models.UnifiedMessage{
		{Role: "user", Content: scenario.Question},
	}

	const maxStep = 10
	for step := 0; step < maxStep; step++ {
		req := &models.UnifiedRequest{
			Model:     chatModel.Model,
			Messages:  messages,
			Stream:    false,
			MaxTokens: 1024,
			Tools:     tools,
		}

		body, err := anthropicsvc.TransformFromUnified(req)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("构建 Anthropic 请求失败: %w", err)
		}

		httpReq, err := providerInstance.BuildReq(ctx, header, chatModel.Model, body)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("构建请求失败: %w", err)
		}

		res, err := httpClient.Do(httpReq)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("连接提供商失败: %w", err)
		}
		bodyBytes, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			return toolCities, finalText.String(), fmt.Errorf("读取响应失败: %w", readErr)
		}
		if res.StatusCode != http.StatusOK {
			return toolCities, finalText.String(), errors.New(BuildDetailedError(getErrorTypeFromStatus(res.StatusCode), "提供商返回错误", parseProviderErrorDetail(bodyBytes), map[string]string{
				"provider":    chatModel.Name,
				"model":       chatModel.Model,
				"status_code": strconv.Itoa(res.StatusCode),
			}))
		}

		parsed, err := anthropicsvc.ParseResponse(bodyBytes)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("解析 Anthropic 响应失败: %w", err)
		}
		if parsed == nil || len(parsed.Choices) == 0 || parsed.Choices[0].Message == nil {
			return toolCities, finalText.String(), errors.New("提供商响应为空")
		}

		assistant := parsed.Choices[0].Message
		if content, ok := assistant.Content.(string); ok && strings.TrimSpace(content) != "" {
			finalText.WriteString(content)
			c.SSEvent("message", content)
			c.Writer.Flush()
		}

		messages = append(messages, models.UnifiedMessage{
			Role:      "assistant",
			Content:   assistant.Content,
			ToolCalls: assistant.ToolCalls,
		})

		if len(assistant.ToolCalls) == 0 {
			break
		}

		for _, call := range assistant.ToolCalls {
			toolCallData, _ := json.Marshal(map[string]any{
				"name":      call.Function.Name,
				"arguments": call.Function.Arguments,
			})
			c.SSEvent("toolcall", string(toolCallData))
			c.Writer.Flush()

			if call.Function.Name != "get_weather" {
				return toolCities, finalText.String(), fmt.Errorf("invalid tool call name: %s", call.Function.Name)
			}

			city, toolRes := weatherFromArguments(call.Function.Arguments)
			toolCities = append(toolCities, city)

			toolResData, _ := json.Marshal(map[string]any{
				"tool_use_id": call.ID,
				"content":     toolRes,
			})
			c.SSEvent("toolres", string(toolResData))
			c.Writer.Flush()

			messages = append(messages, models.UnifiedMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    toolRes,
			})
		}
	}

	return toolCities, finalText.String(), nil
}

func weatherFromArguments(arguments string) (city string, result string) {
	location := gjson.Get(arguments, "location").String()
	unit := gjson.Get(arguments, "unit").String()
	city = normalizeCity(location)
	res, ok := buildWeatherStub(city, unit)
	if !ok {
		res = "暂不支持该地区天气查询"
	}
	return city, res
}

func buildWeatherToolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]string{
				"type":        "string",
				"description": "The city name",
			},
			"unit": map[string]any{
				"type":        "string",
				"description": "Temperature unit (celsius/fahrenheit)",
				"enum":        []string{"celsius", "fahrenheit"},
			},
		},
		"required": []string{"location"},
	}
}

func selectScenarioIndex(modelID, rawIndex string, scenarioCount int) (int, error) {
	if scenarioCount <= 0 {
		return 0, errors.New("no react scenarios configured")
	}
	if strings.TrimSpace(rawIndex) == "" {
		return stableIndex(modelID, scenarioCount), nil
	}
	parsed, err := strconv.Atoi(rawIndex)
	if err != nil {
		return 0, fmt.Errorf("invalid scenario index: %s", rawIndex)
	}
	if parsed < 0 || parsed >= scenarioCount {
		return 0, fmt.Errorf("scenario index out of range: %d", parsed)
	}
	return parsed, nil
}

func stableIndex(key string, mod int) int {
	if mod <= 0 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return int(hash.Sum32() % uint32(mod))
}

func normalizeCity(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, "市")
	s = strings.ToLower(s)

	switch {
	case strings.Contains(s, "南京") || strings.Contains(s, "nanjing"):
		return "南京"
	case strings.Contains(s, "北京") || strings.Contains(s, "beijing"):
		return "北京"
	case strings.Contains(s, "上海") || strings.Contains(s, "shanghai"):
		return "上海"
	case strings.Contains(s, "广州") || strings.Contains(s, "guangzhou"):
		return "广州"
	case strings.Contains(s, "深圳") || strings.Contains(s, "shenzhen"):
		return "深圳"
	case strings.Contains(s, "杭州") || strings.Contains(s, "hangzhou"):
		return "杭州"
	default:
		return strings.TrimSpace(raw)
	}
}

func validateReactToolCalls(toolCities []string, expectedCities []string) error {
	if len(expectedCities) == 0 {
		return errors.New("no expected cities")
	}
	if len(toolCities) < len(expectedCities) {
		return fmt.Errorf("工具调用次数过少: 期望至少 %d 次, 实际 %d 次", len(expectedCities), len(toolCities))
	}

	expected := make(map[string]int, len(expectedCities))
	for _, city := range expectedCities {
		expected[city] = 0
	}

	unknown := make([]string, 0)
	for _, city := range toolCities {
		if _, ok := expected[city]; ok {
			expected[city]++
			continue
		}
		if strings.TrimSpace(city) != "" {
			unknown = append(unknown, city)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("工具调用包含非预期城市: %s", strings.Join(unknown, ", "))
	}

	missing := make([]string, 0)
	for _, city := range expectedCities {
		if expected[city] == 0 {
			missing = append(missing, city)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("工具调用缺少城市: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateReactFinalResponse(finalResponse string, expectedCities []string) error {
	text := strings.TrimSpace(finalResponse)
	if text == "" {
		return errors.New("模型未返回有效文本内容")
	}
	lower := strings.ToLower(text)

	missing := make([]string, 0)
	for _, city := range expectedCities {
		if !containsCityAlias(lower, city) {
			missing = append(missing, city)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("最终回复缺少城市信息: %s", strings.Join(missing, ", "))
	}

	if !(strings.Contains(text, "天气") || strings.Contains(text, "温度") || strings.Contains(lower, "weather") || strings.Contains(text, "℃") || strings.Contains(text, "°")) {
		return errors.New("最终回复缺少天气相关信息")
	}
	return nil
}

func containsCityAlias(lowerText, canonicalCity string) bool {
	if strings.Contains(lowerText, strings.ToLower(canonicalCity)) {
		return true
	}
	switch canonicalCity {
	case "南京":
		return strings.Contains(lowerText, "nanjing")
	case "北京":
		return strings.Contains(lowerText, "beijing")
	case "上海":
		return strings.Contains(lowerText, "shanghai")
	case "广州":
		return strings.Contains(lowerText, "guangzhou")
	case "深圳":
		return strings.Contains(lowerText, "shenzhen")
	case "杭州":
		return strings.Contains(lowerText, "hangzhou")
	default:
		return false
	}
}

type weatherStub struct {
	Desc string
	Temp int // Celsius
}

var weatherStubs = map[string]weatherStub{
	"南京": {Desc: "晴转多云", Temp: 18},
	"北京": {Desc: "大雨转小雨", Temp: 15},
	"上海": {Desc: "多云", Temp: 22},
	"广州": {Desc: "雷阵雨", Temp: 27},
	"深圳": {Desc: "小雨", Temp: 26},
	"杭州": {Desc: "阴", Temp: 20},
}

func buildWeatherStub(city, unit string) (string, bool) {
	if strings.TrimSpace(city) == "" {
		return "", false
	}
	stub, ok := weatherStubs[city]
	if !ok {
		return "", false
	}

	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "celsius":
		return fmt.Sprintf("%s天气%s，温度 %d℃", city, stub.Desc, stub.Temp), true
	case "fahrenheit":
		tempF := int(float64(stub.Temp)*9.0/5.0 + 32.0)
		return fmt.Sprintf("%s天气%s，温度 %d°F", city, stub.Desc, tempF), true
	default:
		return fmt.Sprintf("%s天气%s，温度 %d℃", city, stub.Desc, stub.Temp), true
	}
}
