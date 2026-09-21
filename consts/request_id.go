package consts

// 请求编号（Stage 3）：中间件为每个请求确定唯一编号，错误日志、响应头、
// 访问日志三处共用同一键，保证「用户报障编号 → 服务端捞日志」一条链对齐。
const (
	// HeaderRequestID 对外协商的请求编号头。与 /v1 透传头白名单中的同名头一致：
	// 调用方自带的编号会被规范化沿用（见 middleware.RequestID）。
	HeaderRequestID = "X-Request-Id"

	// ContextKeyRequestID gin.Context 中存放请求编号的键（字符串值）。
	ContextKeyRequestID = "request_id"

	// 请求编号来源前缀（Stage 17 防串号）：自生成与沿用客户端值打不同前缀，
	// httpresp 文案 / slog 字段 / 日志页回显三处消费同一个带前缀值，
	// 运维按编号 grep 时可区分「网关生成」与「调用方自带」。
	// 前缀进值本体（而非展示点拼接）：access_log formatter 只拿得到请求头，
	// 落库/响应头/前端补号各自拼必然漂移，一处进值处处同串。
	// 入站值已含前缀形态的会被中间件判非法重生成（前缀是网关标记，外部不得自选）。
	RequestIDPrefixGenerated = "gw:"
	RequestIDPrefixInbound   = "in:"
)
