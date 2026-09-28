package chat

import (
	"context"
	"errors"
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

// newGatewayTimeoutError 网关总超时（请求 ctx deadline 到点、服务端掐断）统一走
// 504 Gateway Timeout：语义是「网关等不到完整响应」，与真 500（网关 bug）、502（上游
// 全挂）区分，客户端与监控可分类。静态文案不拼内部错误文本（framework-conventions §5）。
func newGatewayTimeoutError() error {
	return newClientRequestError(http.StatusGatewayTimeout, "gateway total timeout exceeded")
}

// mapAbortErr 把「网关自己掐断」（deadline 到点）映射为自带 504 的超时错误；
// 调用方撤走（Canceled）等其余取消原样透传——ctx 已死说明对面多半已收不到。
// 调用点单源在 BalanceChat 返回处（真实/虚拟两路径全部出口共用），内层禁自映射。
func mapAbortErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return newGatewayTimeoutError()
	}
	return err
}

// retryExhaustedOutcome 穷尽出口的归因修正：末次尝试恰撞 deadline 时重试循环以
// exhausted 收场（已无剩余迭代可走中止分支），本质仍是网关掐断——按超时归因
// （最终经 BalanceChat 唯一出口映射 504），不记「上游全挂」的 502。
func retryExhaustedOutcome(ctx context.Context, detail string) error {
	if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return newRetryExhaustedError(detail)
}
