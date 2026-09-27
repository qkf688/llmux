package testapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	anthropicsvc "github.com/qkf688/llmux/service/anthropic"
)

func runAnthropicReactTest(ctx context.Context, c *gin.Context, httpClient *http.Client, header http.Header, link *testLink, providerInstance providers.Provider, scenario reactScenario) ([]string, string, error) {
	if link.Type != providers.TypeAnthropic {
		return nil, "", fmt.Errorf("invalid provider type for anthropic runner: %s", link.Type)
	}
	// 仅依赖 Provider 接口（BuildReq），不强制 *providers.Anthropic 类型断言。
	if providerInstance == nil {
		return nil, "", fmt.Errorf("invalid provider instance: nil")
	}

	// 空密钥本地守卫（Stage 15 契约补齐 anthropic，2026-09-20 插队修复 find-issues 缺口）：
	// 凭据明文经 CredentialCapable 能力接口从装配好的客户端读取（OCP：不断言具体类型），
	// 判据与文案单源在 emptyCredentialGuard。未实现该接口 = 无法校验凭据配置，宁失败不
	// 放行——放行就退回「配置问题伪装成上游故障」；错误点名缺失接口，修复方式 = provider
	// 实现 providers.CredentialCapable。
	capable, ok := providerInstance.(providers.CredentialCapable)
	if !ok {
		return nil, "", fmt.Errorf("供应商 %q 的上游客户端未实现 providers.CredentialCapable，无法校验凭据配置，ReAct 诊断拒绝放行", link.ProviderName)
	}
	if err := emptyCredentialGuard(link.ProviderName, capable.CredentialPlaintext()); err != nil {
		return nil, "", err
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
			Model:     link.Model,
			Messages:  messages,
			Stream:    false,
			MaxTokens: 1024,
			Tools:     tools,
		}

		body, err := anthropicsvc.TransformFromUnified(req)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("构建 Anthropic 请求失败: %w", err)
		}

		httpReq, err := providerInstance.BuildReq(ctx, header, link.Model, body)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("构建请求失败: %w", err)
		}

		res, err := httpClient.Do(httpReq)
		if err != nil {
			return toolCities, finalText.String(), fmt.Errorf("连接提供商失败: %w", err)
		}
		bodyBytes, readErr := common.ReadCappedBody(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			return toolCities, finalText.String(), fmt.Errorf("读取响应失败: %w", readErr)
		}
		if !common.IsPassingStatus(res.StatusCode) {
			return toolCities, finalText.String(), errors.New(BuildDetailedError(common.ErrorTypeFromStatus(res.StatusCode), "提供商返回错误", common.ExtractUpstreamErrorDetail(bodyBytes), map[string]string{
				"provider":    link.ProviderName,
				"model":       link.Model,
				"group":       link.Group.Name,
				"status_code": strconv.Itoa(res.StatusCode),
			}))
		}

		parsed, err := anthropicsvc.ParseResponse(bodyBytes)
		if err != nil {
			// 带状态码：通过口径带上 201 后，「状态码通过但内容不是 Anthropic 响应」也会走到这里。
			return toolCities, finalText.String(), fmt.Errorf("解析 Anthropic 响应失败 (HTTP %d): %w", res.StatusCode, err)
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

		stepMessages, stepCities, toolErr := emitAnthropicToolFrames(c, marshalFrameJSON, assistant.ToolCalls)
		toolCities = append(toolCities, stepCities...)
		messages = append(messages, stepMessages...)
		if toolErr != nil {
			return toolCities, finalText.String(), toolErr
		}
	}

	return toolCities, finalText.String(), nil
}

// emitAnthropicToolFrames 回写一步内全部工具帧（toolcall / toolres）并产出 tool 消息与城市。
// 停流契约与 openai 侧同一份口径（react_frame.go 的 marshalReactFrame）：任一帧序列化
// 失败立即停帧回写（失败帧自身与后续帧都不进 SSE）并返回错误，交 handler 流尾统一回
// event:error——禁止吞错续帧（修复前两处 json.Marshal 的错误被 `_` 吞掉、空帧照发）。
// 非法工具名同为失败出口；toolcall 帧在名字校验**之前**回写（面板要看到尝试了什么调用，
// 再判失败），失败前已完成的产出（城市/tool 消息）保留返回。
func emitAnthropicToolFrames(c *gin.Context, marshal frameMarshaler, calls []models.UnifiedToolCall) ([]models.UnifiedMessage, []string, error) {
	toolMessages := make([]models.UnifiedMessage, 0, len(calls))
	toolCities := make([]string, 0, len(calls))
	for _, call := range calls {
		toolCallData, err := marshalReactFrame(marshal, map[string]any{
			"name":      call.Function.Name,
			"arguments": call.Function.Arguments,
		})
		if err != nil {
			return toolMessages, toolCities, err
		}
		c.SSEvent("toolcall", toolCallData)
		c.Writer.Flush()

		if call.Function.Name != "get_weather" {
			return toolMessages, toolCities, fmt.Errorf("invalid tool call name: %s", call.Function.Name)
		}

		city, toolRes := weatherFromArguments(call.Function.Arguments)
		toolCities = append(toolCities, city)

		toolResData, err := marshalReactFrame(marshal, map[string]any{
			"tool_use_id": call.ID,
			"content":     toolRes,
		})
		if err != nil {
			return toolMessages, toolCities, err
		}
		c.SSEvent("toolres", toolResData)
		c.Writer.Flush()

		toolMessages = append(toolMessages, models.UnifiedMessage{
			Role:       "tool",
			ToolCallID: call.ID,
			Content:    toolRes,
		})
	}
	return toolMessages, toolCities, nil
}
