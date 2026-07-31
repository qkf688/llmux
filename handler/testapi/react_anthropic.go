package testapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	anthropicsvc "github.com/qkf688/llmux/service/anthropic"
)

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
