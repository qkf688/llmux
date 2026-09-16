package consts

// 请求编号（Stage 3）：中间件为每个请求确定唯一编号，错误日志、响应头、
// 访问日志三处共用同一键，保证「用户报障编号 → 服务端捞日志」一条链对齐。
const (
	// HeaderRequestID 对外协商的请求编号头。与 /v1 透传头白名单中的同名头一致：
	// 调用方自带的编号会被规范化沿用（见 middleware.RequestID）。
	HeaderRequestID = "X-Request-Id"

	// ContextKeyRequestID gin.Context 中存放请求编号的键（字符串值）。
	ContextKeyRequestID = "request_id"
)
