package chat

import (
	"fmt"
	"net/http"
)

// clientRequestError marks an error that should be returned to the caller as-is
// (HTTP status), rather than being treated as a provider failure/retryable error.
type clientRequestError struct {
	statusCode int
	message    string
}

func (e *clientRequestError) Error() string { return e.message }

func (e *clientRequestError) StatusCode() int { return e.statusCode }

func newClientRequestError(statusCode int, message string) error {
	return &clientRequestError{statusCode: statusCode, message: message}
}

// newNoRouteForModelError 配置态失败（该模型凑不出任何可用路由：模型不存在/无关联/
// 全停用/能力筛光/白名单对不上）统一走 404：对齐 OpenAI「model not found」口径，
// 客户端不重试——重试不可能变好。modelName 是调用方自有输入，拼进文案帮助定位。
func newNoRouteForModelError(modelName string) error {
	return newClientRequestError(http.StatusNotFound, "not found model "+modelName)
}

// newRetryExhaustedError 重试穷尽（候选全部失败，含虚拟模型全真实模型穷尽）统一走
// 502 Bad Gateway：语义是「上游全挂」而非「网关自身故障」，与配置态 404 区分开；
// 客户端对 502 与 500 的重试行为一致，监控侧可区分两类故障。detail 仅含候选名等
// 非敏感信息，不拼内部错误文本。
func newRetryExhaustedError(detail string) error {
	return newClientRequestError(http.StatusBadGateway,
		fmt.Sprintf("all providers failed for this request (%s)", detail))
}
