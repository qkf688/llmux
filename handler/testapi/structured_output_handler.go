package testapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/credprobe"
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

	slog.Info("Testing structured output", "proxy", link.Client.GetProxy(), "provider", link.ProviderName, "model", link.Model, "type", link.Type, "group", link.Group.Name)

	fail := func(errorType, summary, detail string, rawOutput ...string) {
		payload := failedTestResult(link, errorType, summary, detail)
		if len(rawOutput) > 0 {
			payload["raw_output"] = rawOutput[0]
		}
		httpresp.Success(c, payload)
	}

	testBody, err := buildStructuredOutputTestBody(link.Type)
	if err != nil {
		fail(common.ErrorTypeValidation, "不支持的提供商类型", err.Error())
		return
	}

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)
	out := credprobe.RunCompletionProbe(credprobe.CompletionProbeRequest{
		Ctx:     ctx,
		Client:  link.Client,
		Timeout: providerTestTimeout,
		Headers: header,
		Model:   link.Model,
		Body:    testBody,
	})
	if !out.Passed {
		// raw_output 口径与改造前一致：拿到上游响应且成功读完（含空 body、非 2xx）才带键；
		// BuildReq/连接/读体失败无原文，不带键。
		if out.Summary == credprobe.ProbeSummaryUpstreamError {
			fail(out.ErrorType, out.Summary, out.Detail, string(out.Body))
		} else {
			fail(out.ErrorType, out.Summary, out.Detail)
		}
		return
	}
	bodyBytes := out.Body

	// 上游响应体形状由 provider metadata 声明；未声明时传零值，落到
	// extractStructuredOutputJSON 的 default 分支（按 OpenAI Chat 形状取），与改造前一致。
	// 类型取链路的注册类型（按端点协议派生，与 chat 主链路同源），不再取 Provider.Type。
	upstreamFormat, _ := providers.WireFormatOf(link.Type)
	payload, rawOutput, err := extractStructuredOutputJSON(upstreamFormat, bodyBytes)
	if err != nil {
		fail(common.ErrorTypeValidation, "提取结构化输出失败", err.Error(), string(bodyBytes))
		return
	}

	parsed, err := validateStructuredOutputPayload(payload)
	if err != nil {
		fail(common.ErrorTypeValidation, "结构化输出验证失败", err.Error(), rawOutput)
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
