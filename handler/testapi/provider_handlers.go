package testapi

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/service/credprobe"
)

// 诊断 completion 探针的上游超时见 common.DiagnosticConnectivityTestTimeout（诊断时间预算
// 单源，Stage 19）。ctx deadline 早于 transport 的超时分级机制（network vs timeout）在
// credprobe.RunCompletionProbe 内统一实现。

// ProviderTestHandler 关联级连通性测试（GET /api/test/:id，id = ModelWithProvider 主键）。
//
// 结果形状见 respond.go：链路解析失败按 400/404 或 passed=false 回包，测试本身
// （含上游未通过判定）一律 code 200 + data.passed。
//
// 与三个能力测试共用 runCompletionTestFlow（此前自装一套编排，是同一份链路代码的第二副本）。
// 唯一的形状差异是成功回包把上游原文放在 message 而非 raw_output——那是 /api/test/:id 的既有
// 契约（前端 ExpandableError 直接吃 message），保留不变。
//
// 本端点是四个测试端点里唯一「不判领域内容」的，因此 ?stream=1 时它天然就是
// 「这条链路能不能正常流式」的探测器（严格三门见 completion_flow.go）。
func ProviderTestHandler(c *gin.Context) {
	stream := queryStreamMode(c)
	// fail 用不到：本端点没有领域判定，流里能到这里就说明探针已过（流式时含三门校验）。
	link, outcome, _, ok := runCompletionTestFlow(c, "provider", buildTestBody, stream)
	if !ok {
		return
	}

	result := passedTestResult(link)
	applyOutcomeFields(result, outcome)
	httpresp.SuccessWithMessage(c, outcome.rawText, result)
}

// ProviderModelTestHandler 供应商级模型测试（POST /api/providers/:id/test），供两个模型
// 弹窗的「测试」按钮与批量测试使用。
//
// 请求体的 group_id 见 ProviderModelTestRequest：0 = 自动口径，>0 = 锁定该分组。
// 响应恒回传实际命中的分组 id+name（含失败分支），前端据此把 ✓/✕ 归位到正确徽标。
// 探针编排单一来源见 credprobe.RunCompletionProbe。
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
	link, err := ResolveProviderLink(ctx, id, strings.TrimSpace(req.Model), req.GroupID)
	if err != nil {
		respondResolveFailure(c, err, "Provider not found")
		return
	}

	slog.Info("Testing provider model", "provider", link.ProviderName, "model", link.Model, "group", link.Group.Name)

	testBody, err := buildTestBody(link.Type)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, common.ErrorTypeValidation, "不支持的提供商类型", err.Error()))
		return
	}

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)
	out := credprobe.RunCompletionProbe(credprobe.CompletionProbeRequest{
		Ctx:     ctx,
		Client:  link.Client,
		Timeout: common.DiagnosticConnectivityTestTimeout,
		Headers: header,
		Model:   link.Model,
		Body:    testBody,
	})
	if !out.Passed {
		httpresp.Success(c, failedTestResult(link, out.ErrorType, out.Summary, out.Detail))
		return
	}

	httpresp.SuccessWithMessage(c, string(out.Body), passedTestResult(link))
}
