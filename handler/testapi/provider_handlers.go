package testapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
)

// ProviderTestHandler 关联级连通性测试（GET /api/test/:id，id = ModelWithProvider 主键）。
//
// 结果形状见 respond.go：链路解析失败按 400/404 或 passed=false 回包，测试本身
// （含上游非 200）一律 code 200 + data.passed。
func ProviderTestHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}
	ctx := c.Request.Context()

	link, err := ResolveAssociationLink(ctx, id)
	if err != nil {
		respondResolveFailure(c, err, "ModelWithProvider not found")
		return
	}

	proxyURL := link.Client.GetProxy()
	slog.Info("Testing provider", "proxy", proxyURL, "provider", link.ProviderName, "group", link.Group.Name)
	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)

	testBody, err := buildTestBody(link.Type)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, "validation", "不支持的提供商类型", err.Error()))
		return
	}

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)
	req, err := link.Client.BuildReq(ctx, header, link.Model, testBody)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, "network", "构建请求失败", err.Error()))
		return
	}

	res, err := httpClient.Do(req)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, "network", "连接提供商失败", err.Error()))
		return
	}
	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		httpresp.Success(c, failedTestResult(link, "network", "读取响应失败", readErr.Error()))
		return
	}

	if res.StatusCode != http.StatusOK {
		errorType := getErrorTypeFromStatus(res.StatusCode)
		httpresp.Success(c, failedTestResult(link, errorType, "提供商返回错误", upstreamFailDetail(res.StatusCode, bodyBytes)))
		return
	}

	httpresp.SuccessWithMessage(c, string(bodyBytes), passedTestResult(link))
}

// ProviderModelTestHandler 供应商级模型测试（POST /api/providers/:id/test），供两个模型
// 弹窗的「测试」按钮与批量测试使用。
//
// 请求体的 group_id 见 ProviderModelTestRequest：0 = 自动口径，>0 = 锁定该分组。
// 响应恒回传实际命中的分组 id+name（含失败分支），前端据此把 ✓/✕ 归位到正确徽标。
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
	httpClient := providers.GetClientWithProxy(time.Second*60, link.Client.GetProxy())

	testBody, err := buildTestBody(link.Type)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, "validation", "不支持的提供商类型", err.Error()))
		return
	}

	header := BuildTestHeaders(c.Request.Header, link.WithHeader, link.CustomerHeaders)
	testReq, err := link.Client.BuildReq(ctx, header, link.Model, testBody)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, "network", "构建请求失败", err.Error()))
		return
	}

	res, err := httpClient.Do(testReq)
	if err != nil {
		httpresp.Success(c, failedTestResult(link, "network", "连接提供商失败", err.Error()))
		return
	}
	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		httpresp.Success(c, failedTestResult(link, "network", "读取响应失败", readErr.Error()))
		return
	}

	if res.StatusCode != http.StatusOK {
		errorType := getErrorTypeFromStatus(res.StatusCode)
		httpresp.Success(c, failedTestResult(link, errorType, "提供商返回错误", upstreamFailDetail(res.StatusCode, bodyBytes)))
		return
	}

	httpresp.SuccessWithMessage(c, string(bodyBytes), passedTestResult(link))
}
