package testapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
)

func TestReactHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	link, err := ResolveAssociationLink(ctx, id)
	if err != nil {
		// 与改造前一致：记录/ID 问题回 JSON 信封，链路装配问题回 SSE error 事件
		// （前端 fetchEventSource 按事件流消费，两类事实各走自己的通道）。
		if respondRequestError(c, err, "ModelWithProvider not found") {
			return
		}
		c.SSEvent("error", "装配上游链路失败: "+err.Error())
		c.Writer.Flush()
		return
	}

	scenarioIndex, err := selectScenarioIndex(id, c.Query("scenario"), len(reactScenarios))
	if err != nil {
		c.SSEvent("error", err.Error())
		c.Writer.Flush()
		return
	}
	scenario := reactScenarios[scenarioIndex]

	providerInstance := link.Client
	proxyURL := providerInstance.GetProxy()
	httpClient := providers.GetClientWithProxy(common.DiagnosticReactHeaderTimeout, proxyURL)

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)

	var checkError error
	toolCities := make([]string, 0)
	finalText := strings.Builder{}

	c.SSEvent("start", fmt.Sprintf("提供商:%s 模型:%s 类型:%s 分组:%s 场景:%d 问题:%s", link.ProviderName, link.Model, link.Type, link.Group.Name, scenarioIndex, scenario.Question))
	c.Writer.Flush()
	start := time.Now()

	// OCP-9：优先 OpenAICompat 能力接口；否则 anthropic 协议族；禁止 *OpenAI/*OpenAIRes/*Anthropic 类型断言。
	if compat, ok := providerInstance.(providers.OpenAICompat); ok {
		var final string
		toolCities, final, checkError = runOpenAIReactTest(ctx, c, httpClient, header, link, compat, scenario)
		finalText.WriteString(final)
	} else if link.Type == providers.TypeAnthropic {
		var final string
		toolCities, final, checkError = runAnthropicReactTest(ctx, c, httpClient, header, link, providerInstance, scenario)
		finalText.WriteString(final)
	} else {
		c.SSEvent("error", fmt.Sprintf("该测试仅支持 OpenAI 兼容接口（%s/%s）或 %s，当前提供商类型: %s", providers.TypeOpenAI, providers.TypeOpenAIRes, providers.TypeAnthropic, link.Type))
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

// emptyCredentialGuard ReAct 诊断空密钥本地守卫（Stage 15 契约，2026-09-20 补齐 anthropic）：
// 凭据明文为空 = 配置错误而非供应商故障——诊断必须本地直接给出可执行结论，不带空凭据头
// 打上游挨 401（那会把配置问题伪装成上游故障，管理员看到的是上游原文而非「该配 key 了」）。
// 判据与文案单源：openai 系（compat.OpenAICompatAPIKey）与 anthropic（providers.
// CredentialCapable 读明文）两条 runner 共用本函数；system-ops.md ReAct 契约句引用同一措辞。
func emptyCredentialGuard(providerName, apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("供应商 %q 未配置 API Key：请先在该供应商的凭据分组或号池中配置密钥", providerName)
	}
	return nil
}
