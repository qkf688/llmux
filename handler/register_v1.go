package handler

import (
	"github.com/atopos31/llmio/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterV1 注册 /v1 协议路由（OpenAI / Anthropic）。
func RegisterV1(v1 *gin.RouterGroup, d Deps) {
	authOpenAI := middleware.Auth(d.Token)
	authAnthropic := middleware.AuthAnthropic(d.Token)

	v1.GET("/models", authOpenAI, ModelsHandler)
	v1.POST("/chat/completions", authOpenAI, ChatCompletionsHandler)
	v1.POST("/responses", authOpenAI, ResponsesHandler)
	v1.POST("/messages", authAnthropic, Messages)
	// TODO
	v1.POST("/count_tokens", authAnthropic)
}