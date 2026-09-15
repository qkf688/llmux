package handler

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/qkf688/llmux/handler/v1"
	"github.com/qkf688/llmux/middleware"
)

// RegisterV1 注册 /v1 协议路由（OpenAI / Anthropic），使用调用方 API key 鉴权（api_keys 表多 key）。
func RegisterV1(rg *gin.RouterGroup, d Deps) {
	authAPIKey := middleware.AuthAPIKey(Repos().ApiKey, Repos().User)

	rg.GET("/models", authAPIKey, v1.ModelsHandler)
	rg.POST("/chat/completions", authAPIKey, v1.ChatCompletionsHandler)
	rg.POST("/responses", authAPIKey, v1.ResponsesHandler)
	rg.POST("/messages", authAPIKey, v1.Messages)
	rg.POST("/count_tokens", authAPIKey, v1.CountTokens)
}
