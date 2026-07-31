package testapi

import (
	"context"
	"encoding/json"
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
func runOpenAIReactTest(ctx context.Context, c *gin.Context, httpClient *http.Client, header http.Header, chatModel *ChatModel, compat providers.OpenAICompat, scenario reactScenario) ([]string, string, error) {
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

	var checkError error
	toolCities := make([]string, 0)
	finalText := strings.Builder{}

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

	return toolCities, finalText.String(), checkError
}
