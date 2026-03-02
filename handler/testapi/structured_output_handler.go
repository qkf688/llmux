package testapi

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/providers"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestStructuredOutputHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	chatModel, err := FindChatModel(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "ModelWithProvider not found")
			return
		}
		common.InternalServerError(c, "Database error")
		return
	}

	providerInstance, err := providers.New(chatModel.Type, chatModel.Config, chatModel.Proxy)
	if err != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error":  "创建提供商失败: " + err.Error(),
		})
		return
	}

	proxyURL := providerInstance.GetProxy()
	slog.Info("Testing structured output", "proxy", proxyURL, "provider", chatModel.Name, "model", chatModel.Model, "type", chatModel.Type)
	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)

	testBody, err := buildStructuredOutputTestBody(chatModel.Type)
	if err != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("validation", "不支持的提供商类型", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
		})
		return
	}

	header := BuildTestHeaders(c.Request.Header, chatModel.WithHeader, chatModel.CustomerHeaders)
	req, err := providerInstance.BuildReq(ctx, header, chatModel.Model, testBody)
	if err != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("network", "构建请求失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
		})
		return
	}

	res, err := httpClient.Do(req)
	if err != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("network", "连接提供商失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
				"proxy":    proxyURL,
			}),
		})
		return
	}
	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("network", "读取响应失败", readErr.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
		})
		return
	}

	if res.StatusCode != http.StatusOK {
		errorDetail := parseProviderErrorDetail(bodyBytes)
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError(getErrorTypeFromStatus(res.StatusCode), "提供商返回错误", errorDetail, map[string]string{
				"provider":    chatModel.Name,
				"model":       chatModel.Model,
				"type":        chatModel.Type,
				"status_code": strconv.Itoa(res.StatusCode),
			}),
			"raw_output": string(bodyBytes),
		})
		return
	}

	payload, rawOutput, err := extractStructuredOutputJSON(chatModel.Type, bodyBytes)
	if err != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("validation", "提取结构化输出失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"raw_output": string(bodyBytes),
		})
		return
	}

	parsed, err := validateStructuredOutputPayload(payload)
	if err != nil {
		common.Success(c, map[string]interface{}{
			"passed": false,
			"error": BuildDetailedError("validation", "结构化输出验证失败", err.Error(), map[string]string{
				"provider": chatModel.Name,
				"model":    chatModel.Model,
				"type":     chatModel.Type,
			}),
			"raw_output": rawOutput,
		})
		return
	}

	common.Success(c, map[string]interface{}{
		"passed":        true,
		"message":       "结构化输出能力测试通过",
		"provider":      chatModel.Name,
		"model":         chatModel.Model,
		"provider_type": chatModel.Type,
		"raw_output":    rawOutput,
		"parsed":        parsed,
	})
}
