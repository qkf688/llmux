package testapi

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/httpresp"
)

// TestStructuredOutputHandler 结构化输出能力测试（GET /api/test/structured-output/:id）。
//
// 公共流程（链路装配、探针、流式折叠与严格门、失败回包）在 runCompletionTestFlow；
// 本 handler 只做领域判定：从上游响应提取 JSON 并按固定 schema 校验。
func TestStructuredOutputHandler(c *gin.Context) {
	stream := queryStreamMode(c)
	link, outcome, fail, ok := runCompletionTestFlow(c, "structured output", buildStructuredOutputTestBody, stream)
	if !ok {
		return
	}

	// outcome.format 由链路的注册类型派生；公共流已保证它已知（未声明形状在探针之前就
	// 按 provider 错误回包），不会再静默落到 extractStructuredOutputJSON 的 default 分支。
	payload, rawOutput, err := extractStructuredOutputJSON(outcome.format, outcome.body)
	if err != nil {
		fail(common.ErrorTypeValidation, "提取结构化输出失败", err.Error(), outcome.rawText)
		return
	}

	parsed, err := validateStructuredOutputPayload(payload)
	if err != nil {
		fail(common.ErrorTypeValidation, "结构化输出验证失败", err.Error(), rawOutput)
		return
	}

	result := passedCapabilityResult(link, outcome, "结构化输出能力测试通过"+modeSuffix(stream), map[string]any{
		// raw_output 刻意是提取出的 JSON 负载而非上游整包：本端点的判定对象就是那段负载，
		// 整包（流式下为 SSE 原文）在失败路径上才更需要，见上面两处 fail。
		"raw_output": rawOutput,
		"parsed":     parsed,
	})
	httpresp.Success(c, result)
}
