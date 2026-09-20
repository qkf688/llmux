package testapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/atopos31/nsxno/react"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/qkf688/llmux/providers"
	"github.com/tidwall/gjson"
)

// runOpenAIReactTest 走 OpenAI 兼容能力接口跑 react 流程，SSE 逐块回写。
// 返回工具调用命中的城市、最终文本与过程中的错误。
//
// recover 兜底：第三方库 nsxno/react 在上游对流式请求回非 SSE/空 choices 的响应体时，
// 内部对空切片取下标 panic（agent.go）。属上游不符合流式约定触发的库缺陷、非本地逻辑
// 错误，库不可控故在此兜底转为 checkError——走 handler 既有 SSE error 事件通道，
// 管理端看到的是可读失败事件，而不是 panic 把已开始的流硬切断后「卡住后失败」。
// 契约测试：TestTestReactHandler_NonSSEUpstreamEmitsErrorEvent。
func runOpenAIReactTest(ctx context.Context, c *gin.Context, httpClient *http.Client, header http.Header, link *testLink, compat providers.OpenAICompat, scenario reactScenario) (cities []string, final string, err error) {
	defer func() {
		if r := recover(); r != nil {
			cities, final, err = nil, "", fmt.Errorf("react 流程异常中止（上游响应可能不符合 SSE 流式约定）: %v", r)
		}
	}()
	baseURL, apiKey := compat.OpenAICompatBaseURL(), compat.OpenAICompatAPIKey()
	// 空密钥 = 配置错误而非供应商故障（Stage 15）：本地直接给出可执行结论，
	// 不带空凭据头打上游挨 401——那是把配置问题伪装成上游故障。
	if strings.TrimSpace(apiKey) == "" {
		return nil, "", fmt.Errorf("供应商 %q 未配置 API Key：请先在该供应商的凭据分组或号池中配置密钥", link.ProviderName)
	}
	clientOptions := []option.RequestOption{
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
	}
	for key, values := range header {
		for _, value := range values {
			clientOptions = append(clientOptions, option.WithHeaderAdd(key, value))
		}
	}
	// 凭据头单头保证（4b 契约，Stage 15 方言贯通）：自定义头通道 customer_headers 不限白名单、
	// SDK 环境变量默认值（OPENAI_API_KEY，openai-go 默认选项先于用户选项应用）也可能注入
	// Authorization——先 Del 两个凭据头键清掉全部带入，再搬运 OpenAICompatAuthHeaders 按
	// 供应商方言（config 键 openai_auth_type）写好的凭据头。方言判断单源在
	// providers.applyAuthHeader，本函数只搬运、不 switch 方言，诊断与真实转发永不分叉
	// （修复前 SDK WithAPIKey 恒发 bearer，x-api-key 方言中转在此必假阴性——测坏≠能跑）。
	clientOptions = append(clientOptions,
		option.WithHeaderDel("Authorization"),
		option.WithHeaderDel("x-api-key"),
	)
	for name, values := range compat.OpenAICompatAuthHeaders() {
		for _, value := range values {
			clientOptions = append(clientOptions, option.WithHeader(name, value))
		}
	}

	client := openai.NewClient(clientOptions...)
	agent := react.New(client, 20)
	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "get_weather",
			Description: openai.String("Get weather at the given location"),
			Parameters:  openai.FunctionParameters(buildWeatherToolSchema()),
		}),
	}

	var checkError error
	toolCities := make([]string, 0)
	finalText := strings.Builder{}

	for content, err := range agent.RunStream(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(scenario.Question),
		},
		Tools: tools,
		Model: link.Model,
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

	return toolCities, finalText.String(), checkError
}
