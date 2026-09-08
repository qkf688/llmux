package testapi

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
)

// TestStructuredOutputHandler 结构化输出能力测试（GET /api/test/structured-output/:id）。
//
// 上游链路一律由 ResolveAssociationLink 装配（#19：直用 Provider.Config 会让号池/内联
// 凭据的供应商恒 401）；结果形状与另两个 JSON 测试端点一致，见 respond.go。
func TestStructuredOutputHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	link, err := ResolveAssociationLink(ctx, id)
	if err != nil {
		respondResolveFailure(c, err, "ModelWithProvider not found")
		return
	}

	proxyURL := link.Client.GetProxy()
	slog.Info("Testing structured output", "proxy", proxyURL, "provider", link.ProviderName, "model", link.Model, "type", link.Type, "group", link.Group.Name)
	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)

	fail := func(errorType, summary, detail string, rawOutput ...string) {
		payload := failedTestResult(link, errorType, summary, detail)
		if len(rawOutput) > 0 {
			payload["raw_output"] = rawOutput[0]
		}
		httpresp.Success(c, payload)
	}

	testBody, err := buildStructuredOutputTestBody(link.Type)
	if err != nil {
		fail("validation", "不支持的提供商类型", err.Error())
		return
	}

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)
	req, err := link.Client.BuildReq(ctx, header, link.Model, testBody)
	if err != nil {
		fail("network", "构建请求失败", err.Error())
		return
	}

	res, err := httpClient.Do(req)
	if err != nil {
		fail("network", "连接提供商失败", err.Error())
		return
	}
	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		fail("network", "读取响应失败", readErr.Error())
		return
	}

	if res.StatusCode != http.StatusOK {
		fail(getErrorTypeFromStatus(res.StatusCode), "提供商返回错误",
			upstreamFailDetail(res.StatusCode, bodyBytes), string(bodyBytes))
		return
	}

	// 上游响应体形状由 provider metadata 声明；未声明时传零值，落到
	// extractStructuredOutputJSON 的 default 分支（按 OpenAI Chat 形状取），与改造前一致。
	// 类型取链路的注册类型（按端点协议派生，与 chat 主链路同源），不再取 Provider.Type。
	upstreamFormat, _ := providers.WireFormatOf(link.Type)
	payload, rawOutput, err := extractStructuredOutputJSON(upstreamFormat, bodyBytes)
	if err != nil {
		fail("validation", "提取结构化输出失败", err.Error(), string(bodyBytes))
		return
	}

	parsed, err := validateStructuredOutputPayload(payload)
	if err != nil {
		fail("validation", "结构化输出验证失败", err.Error(), rawOutput)
		return
	}

	result := passedTestResult(link)
	result["message"] = "结构化输出能力测试通过"
	result["provider"] = link.ProviderName
	result["model"] = link.Model
	result["provider_type"] = link.Type
	result["raw_output"] = rawOutput
	result["parsed"] = parsed
	httpresp.Success(c, result)
}
