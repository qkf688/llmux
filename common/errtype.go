package common

// 诊断类测试（testapi 连通测试、#24-B 按凭据测试）共用的 error_type 值域。
// 这一组机器码是前后端契约：镜像 webui/src/lib/errors.ts 的 ERROR_TYPES，两侧改动必须同步。
const (
	ErrorTypeNetwork    = "network"
	ErrorTypeAuth       = "auth"
	ErrorTypeProvider   = "provider"
	ErrorTypeTimeout    = "timeout"
	ErrorTypeValidation = "validation"
	ErrorTypeUnknown    = "unknown"
)

// ErrorTypeFromStatus 把上游 HTTP 状态码映射到诊断测试的 error_type。未通过判定
// （口径见 IsPassingStatus）的「打到了上游、上游回了错误」结论都走这里，产出
// auth / provider / validation / unknown。
//
// network / timeout 不在此产出——它们来自 http.Client.Do 的连接失败 / 上下文超时
// （没有状态码可判），统一由 `service/credprobe.RunCompletionProbe` 在 Do 错误分支用
// errors.Is(DeadlineExceeded) 区分；react 多轮 loop 因流程不同仍各自判定。
//
// 单一来源：值域曾散落在 handler/testapi，#24-B 若另写一份 switch 必与 testapi 漂移、
// 并让前端镜像的 ERROR_TYPES 对不上，故下沉到 common 供两域共用。
func ErrorTypeFromStatus(statusCode int) string {
	switch {
	case statusCode == 401 || statusCode == 403:
		return ErrorTypeAuth
	case statusCode == 404:
		return ErrorTypeProvider
	case statusCode >= 400 && statusCode < 500:
		return ErrorTypeValidation
	case statusCode >= 500:
		return ErrorTypeProvider
	default:
		return ErrorTypeUnknown
	}
}
