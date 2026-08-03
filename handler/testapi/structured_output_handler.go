package testapi

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
	"gorm.io/gorm"
)

func TestStructuredOutputHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

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
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("provider", "创建提供商失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"error_type": "provider",
		})
		return
	}

	proxyURL := providerInstance.GetProxy()
	slog.Info("Testing structured output", "proxy", proxyURL, "provider", chatModel.Name, "model", chatModel.Model, "type", chatModel.Type)
	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)

	testBody, err := buildStructuredOutputTestBody(chatModel.Type)
	if err != nil {
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("validation", "不支持的提供商类型", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"error_type": "validation",
		})
		return
	}

	header := BuildTestHeaders(c.Request.Header, chatModel.WithHeader, chatModel.CustomerHeaders)
	req, err := providerInstance.BuildReq(ctx, header, chatModel.Model, testBody)
	if err != nil {
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("network", "构建请求失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"error_type": "network",
		})
		return
	}

	res, err := httpClient.Do(req)
	if err != nil {
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("network", "连接提供商失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
				"proxy":    proxyURL,
			}),
			"error_type": "network",
		})
		return
	}
	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("network", "读取响应失败", readErr.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"error_type": "network",
		})
		return
	}

	if res.StatusCode != http.StatusOK {
		errorDetail := parseProviderErrorDetail(bodyBytes)
		errorType := getErrorTypeFromStatus(res.StatusCode)
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError(errorType, "提供商返回错误", errorDetail, map[string]string{
				"provider":    chatModel.Name,
				"model":       chatModel.Model,
				"type":        chatModel.Type,
				"status_code": strconv.Itoa(res.StatusCode),
			}),
			"error_type": errorType,
			"raw_output": string(bodyBytes),
		})
		return
	}

	payload, rawOutput, err := extractStructuredOutputJSON(chatModel.Type, bodyBytes)
	if err != nil {
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("validation", "提取结构化输出失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"error_type": "validation",
			"raw_output": string(bodyBytes),
		})
		return
	}

	parsed, err := validateStructuredOutputPayload(payload)
	if err != nil {
		httpresp.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("validation", "结构化输出验证失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"error_type": "validation",
			"raw_output": rawOutput,
		})
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"passed":        true,
		"message":       "结构化输出能力测试通过",
		"provider":      chatModel.Name,
		"model":         chatModel.Model,
		"provider_type": chatModel.Type,
		"raw_output":    rawOutput,
		"parsed":        parsed,
	})
}
