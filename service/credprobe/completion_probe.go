package credprobe

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/providers"
)

// 本文件是「一次真实最小 completion 探针」编排的单一来源（BuildReq → 发请求 →
// Do 错误归类 → 读 body → 未通过判定归类 → 组装失败细节）。曾有三份近似拷贝
// （testapi 两个 provider 测试 handler、pools #24-B 按凭据测试，外加 structured-output
// 共四处），归类/失败细节口径开始分叉后收敛于此——下次加同类测试端点只调本函数。

// 探针失败阶段的用户可见文案（单一来源）：各端点信封形状可以不同，但同一失败阶段
// 不再各写一份措辞。
const (
	ProbeSummaryBuildRequest  = "构建请求失败"
	ProbeSummaryConnect       = "连接上游失败"
	ProbeSummaryReadResponse  = "读取上游响应失败"
	ProbeSummaryUpstreamError = "上游返回错误"
)

// CompletionProbeRequest 一次 completion 探针输入。调用方负责装配 client（指定凭据 /
// 组内首个可用属链路语义，不进探针本体）与 body（探针体形状属 providers 注册表）。
type CompletionProbeRequest struct {
	Ctx    context.Context
	Client providers.Provider
	// Timeout ctx deadline 口径（transport = Timeout+common.DiagnosticTransportBuffer）。
	// 应为常量口径：providers.GetClientWithProxy 的缓存键含 timeout 值，传动态值
	// （如按设置项现算）会让每个不同值都新建一份带独立连接池的 Transport。
	// ≤0 时兜底 common.DiagnosticCompletionTimeout。
	Timeout time.Duration
	Headers http.Header
	Model   string
	Body    []byte
}

// CompletionProbeResult 探针结论。Passed=false 时 ErrorType/Summary/Detail 有值；
// Body 为上游响应体（成功时供回显 message，失败时供日志/raw_output）：不受 Detail 的 ≤512B
// 截断影响，但仍受 common 诊断读体上限（1 MiB）约束——合法探针响应远小于此，仅上游异常回
// 超大响应时才会被截前缀。
type CompletionProbeResult struct {
	Passed     bool
	StatusCode int // 0 = 未打到上游或未拿到响应
	ErrorType  string
	Summary    string
	Detail     string
	Body       []byte
	// Headers 是上游响应头（协议支持的诊断族用：原生厂商响应头是「这条路径是否直通」
	// 的强证据，见 service/protocolprobe）。未打到上游时为 nil。
	Headers http.Header
	// DurationMS 从发请求到读完 body 的耗时，单次样本、只作展示不作排名。
	DurationMS int64
}

// RunCompletionProbe 打一次真实最小 completion 并按统一口径归类结果。
// 通过判定 = 200/201，判据单源 common.IsPassingStatus（Stage 5：健康检查/诊断/真实链路
// 共用同一条口径；带上 201 是因为部分兼容网关会回 201 而非严格 200）。
func RunCompletionProbe(req CompletionProbeRequest) (out CompletionProbeResult) {
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}
	if req.Timeout <= 0 {
		req.Timeout = common.DiagnosticCompletionTimeout
	}

	// 命名返回值 + defer：DurationMS 在每条返回路径上都要填，逐处手写必漏一处
	// （同 stream_probe 的 RunStreamProbe 姿势）。
	start := time.Now()
	defer func() { out.DurationMS = time.Since(start).Milliseconds() }()

	probeCtx, cancel := context.WithTimeout(req.Ctx, req.Timeout)
	defer cancel()

	// Clone：BuildReq 会在传入 map 上就地覆写/删除凭据头（4b 契约，见 providers.Provider），
	// 调用方的 req.Headers 可能被跨凭据/跨组复用，不 Clone 会互相污染（同 stream_probe 姿势）。
	httpReq, err := req.Client.BuildReq(probeCtx, req.Headers.Clone(), req.Model, req.Body)
	if err != nil {
		return CompletionProbeResult{
			ErrorType: common.ErrorTypeNetwork,
			Summary:   ProbeSummaryBuildRequest,
			Detail:    err.Error(),
		}
	}

	httpClient := providers.GetClientWithProxy(req.Timeout+common.DiagnosticTransportBuffer, req.Client.GetProxy())
	res, err := httpClient.Do(httpReq)
	if err != nil {
		// Do 失败没有状态码：区分超时与其他网络错误。
		errType := common.ErrorTypeNetwork
		if errors.Is(err, context.DeadlineExceeded) {
			errType = common.ErrorTypeTimeout
		}
		return CompletionProbeResult{
			ErrorType: errType,
			Summary:   ProbeSummaryConnect,
			Detail:    err.Error(),
		}
	}
	defer res.Body.Close()

	bodyBytes, readErr := common.ReadCappedBody(res.Body)
	if readErr != nil {
		// headers 已回、body 滴流时 deadline 到期同样落在这里（transport 的
		// ResponseHeaderTimeout 管不到读体）：与 Do 分支同款归类，否则「慢上游稳定
		// 归 timeout」的契约从读体路径漏回 network。
		errType := common.ErrorTypeNetwork
		if errors.Is(readErr, context.DeadlineExceeded) {
			errType = common.ErrorTypeTimeout
		}
		return CompletionProbeResult{
			StatusCode: res.StatusCode,
			ErrorType:  errType,
			Summary:    ProbeSummaryReadResponse,
			Detail:     readErr.Error(),
			Headers:    res.Header,
		}
	}

	if !common.IsPassingStatus(res.StatusCode) {
		return CompletionProbeResult{
			StatusCode: res.StatusCode,
			ErrorType:  common.ErrorTypeFromStatus(res.StatusCode),
			Summary:    ProbeSummaryUpstreamError,
			Detail:     common.UpstreamErrorDetail(res.StatusCode, bodyBytes),
			Body:       bodyBytes,
			Headers:    res.Header,
		}
	}

	return CompletionProbeResult{Passed: true, StatusCode: res.StatusCode, Body: bodyBytes, Headers: res.Header}
}
