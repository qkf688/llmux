package testapi

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"gorm.io/gorm"
)

// 本文件收敛「一次诊断测试结果」的回包形状。四个测试端点共用同一形状的原因：
// 前端（webui/src/lib/api 的 ModelProviderTestResult / ProviderModelTestResult 与
// routes/providers 的测试 hook）要能只看 `data.passed` 判定、并把结果按响应回传的
// `group_id` 归位到正确的分组徽标（C2 定案），而 apiRequest 在信封 code!=200 时只抛
// message、丢掉 data——用错误信封回传结论等于前端拿不到分组。

// respondRequestError 若 err 是「调用方或数据问题」（ID 非数字、显式 group_id 不属于
// 该供应商、记录不存在），按既有措辞回 400/404 信封并返回 true；否则原样返回 false，
// 由调用方按自己的通道回包（JSON 端点用 failedTestResult，react 端点用 SSE error 事件）。
func respondRequestError(c *gin.Context, err error, notFoundMsg string) bool {
	switch {
	case errors.Is(err, errInvalidID):
		httpresp.BadRequest(c, "Invalid ID format")
		return true
	case errors.Is(err, errInvalidGroup):
		httpresp.BadRequest(c, err.Error())
		return true
	case errors.Is(err, gorm.ErrRecordNotFound):
		httpresp.NotFound(c, notFoundMsg)
		return true
	}
	return false
}

// respondResolveFailure JSON 型测试端点用的链路解析失败回包：
//   - ID 非法、关联/供应商不存在 → 400/404 信封（调用方或数据问题，不是一次测试结论）；
//   - 其余（无启用端点、无分组、模型不在任何分组白名单、组内无可用凭据、解密失败）→
//     code 200 + `passed:false`，error_type 见 linkErrorType。
//
// react 端点走 SSE，不复用本函数（它只借 respondRequestError）。
func respondResolveFailure(c *gin.Context, err error, notFoundMsg string) {
	if respondRequestError(c, err, notFoundMsg) {
		return
	}
	httpresp.Success(c, failedTestResult(nil, linkErrorType(err), "装配上游链路失败", err.Error()))
}

// testResultBase 结果对象的公共键：passed + 实际命中的分组。
//
// group_id / group_name **恒在**（链路未选定 = 0 与空串），不给「键缺失」留语义：
// 三态在这里没有额外信息量，缺键与 0 都表示「没走到分组这一步」，让前端两套分支去猜
// 反而更容易错。
func testResultBase(link *testLink, passed bool) map[string]any {
	out := map[string]any{"passed": passed}
	if link == nil {
		out["group_id"] = uint(0)
		out["group_name"] = ""
		return out
	}
	out["group_id"] = link.Group.ID
	out["group_name"] = link.Group.Name
	return out
}

// failedTestResult 一次失败的测试：带 error_type（值域同 webui/src/lib/errors.ts 的
// ERROR_TYPES）与人读的 error 文本；链路已选定时把 provider/model/group 写进上下文，
// 未选定时不硬凑（否则日志里出现空 provider 反而误导）。
func failedTestResult(link *testLink, errorType, summary, detail string) map[string]any {
	out := testResultBase(link, false)
	if link == nil {
		out["error"] = BuildDetailedError(errorType, summary, detail, nil)
	} else {
		out["error"] = BuildDetailedError(errorType, summary, detail, map[string]string{
			"provider": link.ProviderName,
			"model":    link.Model,
			"group":    link.Group.Name,
		})
	}
	out["error_type"] = errorType
	return out
}

// passedTestResult 一次通过的测试（上游原文由调用方放进 message，与既有端点一致）。
func passedTestResult(link *testLink) map[string]any {
	return testResultBase(link, true)
}

// upstreamFailDetail 上游非 200 时的详细信息：把状态码并进文本（改造前它由
// ErrorWithHttpStatus 的 code 承载，收敛到 data.error 后不能丢这个事实）。
func upstreamFailDetail(statusCode int, bodyBytes []byte) string {
	return parseProviderErrorDetail(bodyBytes) + "\nHTTP 状态码: " + strconv.Itoa(statusCode)
}
