package handler

import (
	"context"

	"github.com/atopos31/llmio/handler/testapi"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2"
)

// ProviderModelTestRequest 提供商模型测试请求（兼容层）。
type ProviderModelTestRequest = testapi.ProviderModelTestRequest

// ChatModel 模型与提供商组合信息（兼容层）。
type ChatModel = testapi.ChatModel

// ProviderTestHandler 提供商测试接口（兼容层）。
func ProviderTestHandler(c *gin.Context) {
	testapi.ProviderTestHandler(c)
}

// ProviderModelTestHandler 提供商指定模型测试接口（兼容层）。
func ProviderModelTestHandler(c *gin.Context) {
	testapi.ProviderModelTestHandler(c)
}

// TestReactHandler React 工具调用测试接口（兼容层）。
func TestReactHandler(c *gin.Context) {
	testapi.TestReactHandler(c)
}

// TestStructuredOutputHandler 结构化输出能力测试接口（兼容层）。
func TestStructuredOutputHandler(c *gin.Context) {
	testapi.TestStructuredOutputHandler(c)
}

// GetWeather React 测试工具函数（兼容层）。
func GetWeather(ctx context.Context, call openai.ChatCompletionChunkChoiceDeltaToolCallFunction) (*openai.ChatCompletionToolMessageParamContentUnion, error) {
	return testapi.GetWeather(ctx, call)
}

// FindChatModel 查询关联的提供商模型（兼容层）。
func FindChatModel(ctx context.Context, id string) (*ChatModel, error) {
	return testapi.FindChatModel(ctx, id)
}
