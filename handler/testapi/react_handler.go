package testapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
	"gorm.io/gorm"
)

func TestReactHandler(c *gin.Context) {
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

	scenarioIndex, err := selectScenarioIndex(id, c.Query("scenario"), len(reactScenarios))
	if err != nil {
		c.SSEvent("error", err.Error())
		c.Writer.Flush()
		return
	}
	scenario := reactScenarios[scenarioIndex]

	providerInstance, err := providers.New(chatModel.Type, chatModel.Config, chatModel.Proxy)
	if err != nil {
		c.SSEvent("error", "创建提供商失败: "+err.Error())
		c.Writer.Flush()
		return
	}

	proxyURL := providerInstance.GetProxy()
	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)

	header := BuildTestHeaders(c.Request.Header, chatModel.WithHeader, chatModel.CustomerHeaders)

	var checkError error
	toolCities := make([]string, 0)
	finalText := strings.Builder{}

	c.SSEvent("start", fmt.Sprintf("提供商:%s 模型:%s 类型:%s 场景:%d 问题:%s", chatModel.Name, chatModel.Model, chatModel.Type, scenarioIndex, scenario.Question))
	c.Writer.Flush()
	start := time.Now()

	// OCP-9：优先 OpenAICompat 能力接口；否则 anthropic 协议族；禁止 *OpenAI/*OpenAIRes/*Anthropic 类型断言。
	if compat, ok := providerInstance.(providers.OpenAICompat); ok {
		var final string
		toolCities, final, checkError = runOpenAIReactTest(ctx, c, httpClient, header, chatModel, compat, scenario)
		finalText.WriteString(final)
	} else if chatModel.Type == consts.StyleAnthropic {
		var final string
		toolCities, final, checkError = runAnthropicReactTest(ctx, c, httpClient, header, chatModel, providerInstance, scenario)
		finalText.WriteString(final)
	} else {
		c.SSEvent("error", fmt.Sprintf("该测试仅支持 OpenAI 兼容接口（%s/%s）或 %s，当前提供商类型: %s", consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic, chatModel.Type))
		c.Writer.Flush()
		return
	}

	if checkError == nil {
		if err := validateReactToolCalls(toolCities, scenario.ExpectedCities); err != nil {
			checkError = err
		}
	}
	if checkError == nil {
		if err := validateReactFinalResponse(finalText.String(), scenario.ExpectedCities); err != nil {
			checkError = err
		}
	}

	if checkError != nil {
		c.SSEvent("error", checkError.Error())
		c.Writer.Flush()
		return
	}

	c.SSEvent("success", fmt.Sprintf("成功通过测试, 耗时: %.2fs", time.Since(start).Seconds()))
	c.Writer.Flush()
}
