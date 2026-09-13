package testapi

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
)

// TestStructuredOutputHandler 结构化输出能力测试（GET /api/test/structured-output/:id）。
//
// 公共流程（链路装配、探针、失败回包）在 runCompletionTestFlow；本 handler 只做领域判定：
// 从上游响应提取 JSON 并按固定 schema 校验。
func TestStructuredOutputHandler(c *gin.Context) {
	link, bodyBytes, fail, ok := runCompletionTestFlow(c, "structured output", buildStructuredOutputTestBody)
	if !ok {
		return
	}

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
