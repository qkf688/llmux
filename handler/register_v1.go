package handler

import (
	v1 "github.com/atopos31/llmio/handler/v1"
	"github.com/atopos31/llmio/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterV1 注册 /v1 协议路由（OpenAI / Anthropic）。
func RegisterV1(rg *gin.RouterGroup, d Deps) {
	authOpenAI := middleware.Auth(d.Token)
	authAnthropic := middleware.AuthAnthropic(d.Token)

	rg.GET("/models", authOpenAI, v1.ModelsHandler)
	rg.POST("/chat/completions", authOpenAI, v1.ChatCompletionsHandler)
	rg.POST("/responses", authOpenAI, v1.ResponsesHandler)
	rg.POST("/messages", authAnthropic, v1.Messages)
	// TODO
	rg.POST("/count_tokens", authAnthropic)
}
