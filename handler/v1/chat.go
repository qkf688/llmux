package v1

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service"
	"gorm.io/gorm"
)

// ModelsHandler 列出当前可用模型，直接从数据库读取基础信息并按 OpenAI 协议返回。
func ModelsHandler(c *gin.Context) {
	// 获取真实模型
	llmModels, err := gorm.G[models.Model](models.DB).Find(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	modelsList := make([]providers.Model, 0)
	for _, llmModel := range llmModels {
		modelsList = append(modelsList, providers.Model{
			ID:      llmModel.Name,
			Object:  "model",
			Created: llmModel.CreatedAt.Unix(),
			OwnedBy: "llmux",
		})
	}

	// 获取虚拟模型
	virtualModels, err := gorm.G[models.VirtualModel](models.DB).Find(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	// 添加虚拟模型到列表
	for _, virtualModel := range virtualModels {
		modelsList = append(modelsList, providers.Model{
			ID:      virtualModel.Name,
			Object:  "model",
			Created: virtualModel.CreatedAt.Unix(),
			OwnedBy: "llmux-virtual",
		})
	}

	httpresp.SuccessRaw(c, providers.ModelList{
		Object: "list",
		Data:   modelsList,
	})
}

func ChatCompletionsHandler(c *gin.Context) {
	chatHandlerByStyle(c, consts.StyleOpenAI)
}

func ResponsesHandler(c *gin.Context) {
	chatHandlerByStyle(c, consts.StyleOpenAIRes)
}

func Messages(c *gin.Context) {
	chatHandlerByStyle(c, consts.StyleAnthropic)
}

// CountTokens 是 /v1/count_tokens 的占位 handler，返回 501 Not Implemented。
// 该端点仅挂鉴权中间件，待实现真实 token 计数逻辑时替换。
func CountTokens(c *gin.Context) {
	httpresp.ErrorWithHttpStatus(c, http.StatusNotImplemented, http.StatusNotImplemented, "count_tokens not implemented")
}

func chatHandlerByStyle(c *gin.Context, style string) {
	beforer, err := service.GetBeforer(style)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}
	processer, err := service.GetProcesser(style)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}
	chatHandler(c, beforer, processer, style)
}

func chatHandler(c *gin.Context, preProcessor service.Beforer, postProcessor service.Processer, style string) {
	// 读取原始请求体
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	c.Request.Body.Close()
	// 预处理、提取模型参数
	before, err := preProcessor(reqBody)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}
	// 按模型获取可用 provider
	ctx := c.Request.Context()
	providersWithMeta, err := service.ProvidersWithMetaBymodelsName(ctx, style, *before)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	startReq := time.Now()
	// 调用负载均衡后的 provider 并转发
	balanced, err := service.BalanceChat(ctx, service.BalanceInput{
		Start:             startReq,
		Style:             style,
		Before:            *before,
		ProvidersWithMeta: *providersWithMeta,
		ReqMeta: models.ReqMeta{
			Header:    c.Request.Header,
			RemoteIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		},
	})
	if err != nil {
		var statusCoder interface{ StatusCode() int }
		if errors.As(err, &statusCoder) {
			status := statusCoder.StatusCode()
			httpresp.ErrorWithHttpStatus(c, status, status, err.Error())
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}
	res := balanced.Response
	defer res.Body.Close()

	pr, pw := io.Pipe()
	tee := io.TeeReader(res.Body, pw)

	// 使用 WaitGroup 等待日志处理完成，防止pipe过早关闭导致数据丢失
	var wg sync.WaitGroup
	wg.Add(1)

	// 异步处理输出并记录 tokens
	go func() {
		defer wg.Done()
		service.RecordLog(context.Background(), service.RecordLogInput{
			ReqStart:     startReq,
			Reader:       pr,
			Processer:    postProcessor,
			LogID:        balanced.LogID,
			Before:       *before,
			IOLog:        providersWithMeta.IOLog,
			ProviderName: balanced.ProviderName,
			SideChannel:  balanced.SideChannel,
		})
	}()

	writeHeader(c, ctx, before.Stream, res.Header)
	if _, err := io.Copy(c.Writer, tee); err != nil {
		pw.CloseWithError(err)
		httpresp.InternalServerError(c, err.Error())
		return
	}

	// 关闭写入端，让读取端知道数据已发送完
	pw.Close()

	// 等待日志处理完成，确保所有数据都被正确记录
	wg.Wait()
}

func writeHeader(c *gin.Context, ctx context.Context, stream bool, header http.Header) {
	// 检查是否需要移除不必要的响应头
	stripHeaders := service.GetStripResponseHeaders(ctx)

	// 需要保留的核心响应头
	essentialHeaders := map[string]bool{
		"Content-Type":      true,
		"X-Request-Id":      true,
		"X-Ratelimit-Limit": true,
	}

	for k, values := range header {
		// 如果开启了移除响应头选项，只保留必要的头
		if stripHeaders && !essentialHeaders[k] {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(k, value)
		}
	}

	if stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
	}
	c.Writer.Flush()
}
