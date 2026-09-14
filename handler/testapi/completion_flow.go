package testapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/service/credprobe"
)

// runCompletionTestFlow 是所有 JSON 型 completion 测试端点共享的公共流程：
// 解析关联链路 → 取测试体 → credprobe 探针上游。任一失败按 testapi 结果形状回包并返回
// ok=false（调用方直接 return）；探针通过后把链路与上游原始响应体交回，由各端点做
// **领域判定**——提取逻辑与通过标准是端点间唯一的差异点。
// 连通性两个 handler 不走本流：其成功回包形状不同（message=原始响应体，无 raw_output），
// 且探针失败也不带 raw_output（provider_handlers.go 的既有 /test/:id 契约）——统一回包形状前不可并入，
// 否则会静默改变 connectivity 失败分支的回包内容。
// 新增同形状 completion 类测试端点必须走本函数，禁止再抄编排。
func runCompletionTestFlow(c *gin.Context, name string, buildBody func(providerType string) ([]byte, error)) (link *testLink, body []byte, fail func(errorType, summary, detail string, rawOutput ...string), ok bool) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return nil, nil, nil, false
	}

	link, err := ResolveAssociationLink(ctx, id)
	if err != nil {
		respondResolveFailure(c, err, "ModelWithProvider not found")
		return nil, nil, nil, false
	}

	slog.Info("Testing "+name, "proxy", link.Client.GetProxy(), "provider", link.ProviderName, "model", link.Model, "type", link.Type, "group", link.Group.Name)

	fail = func(errorType, summary, detail string, rawOutput ...string) {
		payload := failedTestResult(link, errorType, summary, detail)
		if len(rawOutput) > 0 {
			payload["raw_output"] = rawOutput[0]
		}
		httpresp.Success(c, payload)
	}

	testBody, err := buildBody(link.Type)
	if err != nil {
		fail(common.ErrorTypeValidation, "不支持的提供商类型", err.Error())
		return nil, nil, nil, false
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
		// raw_output 口径：拿到上游响应且成功读完（含空 body、非 2xx）才带键；
		// BuildReq/连接/读体失败无原文，不带键。
		if out.Summary == credprobe.ProbeSummaryUpstreamError {
			fail(out.ErrorType, out.Summary, out.Detail, string(out.Body))
		} else {
			fail(out.ErrorType, out.Summary, out.Detail)
		}
		return nil, nil, nil, false
	}
	return link, out.Body, fail, true
}
