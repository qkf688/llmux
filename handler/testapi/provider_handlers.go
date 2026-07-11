package testapi

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ProviderTestHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}
	ctx := c.Request.Context()

	chatModel, err := FindChatModel(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httpresp.NotFound(c, "ModelWithProvider not found")
			return
		}
		httpresp.InternalServerError(c, "Database error")
		return
	}

	providerInstance, err := providers.New(chatModel.Type, chatModel.Config, chatModel.Proxy)
	if err != nil {
		httpresp.BadRequest(c, "Failed to create provider: "+err.Error())
		return
	}

	proxyURL := providerInstance.GetProxy()
	slog.Info("Testing provider", "proxy", proxyURL, "provider", chatModel.Name)
	client := providers.GetClientWithProxy(time.Second*60, proxyURL)

	testBody, err := buildTestBody(chatModel.Type)
	if err != nil {
		httpresp.BadRequest(c, "Invalid provider type")
		return
	}

	header := BuildTestHeaders(c.Request.Header, chatModel.WithHeader, chatModel.CustomerHeaders)
	req, err := providerInstance.BuildReq(ctx, header, chatModel.Model, testBody)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, http.StatusBadGateway, BuildDetailedError("network", "构建请求失败", err.Error(), map[string]string{
			"provider": chatModel.Name,
			"model":    chatModel.Model,
		}))
		return
	}

	res, err := client.Do(req)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, http.StatusBadGateway, BuildDetailedError("network", "连接提供商失败", err.Error(), map[string]string{
			"provider": chatModel.Name,
			"model":    chatModel.Model,
			"proxy":    proxyURL,
		}))
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		errorDetail := parseProviderErrorDetail(bodyBytes)
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, BuildDetailedError(getErrorTypeFromStatus(res.StatusCode), "提供商返回错误", errorDetail, map[string]string{
			"provider":    chatModel.Name,
			"model":       chatModel.Model,
			"status_code": strconv.Itoa(res.StatusCode),
		}))
		return
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, BuildDetailedError("network", "读取响应失败", err.Error(), map[string]string{
			"provider": chatModel.Name,
			"model":    chatModel.Model,
		}))
		return
	}

	httpresp.SuccessWithMessage(c, string(content), nil)
}

func ProviderModelTestHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	var req ProviderModelTestRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Model) == "" {
		httpresp.BadRequest(c, "Invalid model name")
		return
	}

	ctx := c.Request.Context()
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httpresp.NotFound(c, "Provider not found")
			return
		}
		httpresp.InternalServerError(c, "Database error")
		return
	}

	providerInstance, err := providers.New(provider.Type, provider.Config, provider.Proxy)
	if err != nil {
		httpresp.BadRequest(c, "Failed to create provider: "+err.Error())
		return
	}

	proxyURL := providerInstance.GetProxy()
	slog.Info("Testing provider model", "proxy", proxyURL, "provider", provider.Name, "model", req.Model)
	client := providers.GetClientWithProxy(time.Second*60, proxyURL)

	testBody, err := buildTestBody(provider.Type)
	if err != nil {
		httpresp.BadRequest(c, "Invalid provider type")
		return
	}

	header := BuildTestHeaders(c.Request.Header, nil, nil)
	testReq, err := providerInstance.BuildReq(ctx, header, req.Model, testBody)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, http.StatusBadGateway, "Failed to connect to provider: "+err.Error())
		return
	}

	res, err := client.Do(testReq)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, http.StatusBadGateway, "Failed to connect to provider: "+err.Error())
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, "Provider returned non-200 status code: "+strconv.Itoa(res.StatusCode))
		return
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusOK, res.StatusCode, "Failed to read res body: "+err.Error())
		return
	}

	httpresp.SuccessWithMessage(c, string(content), nil)
}
