package consts

// 请求编号（Stage 3）：中间件为每个请求确定唯一编号，错误日志、响应头、
// 访问日志三处共用同一键，保证「用户报障编号 → 服务端捞日志」一条链对齐。
const (
	// HeaderRequestID 对外协商的请求编号头。与 /v1 透传头白名单中的同名头一致：
	// 调用方自带的编号会被规范化沿用（见 middleware.RequestID）。
	HeaderRequestID = "X-Request-Id"

	// HeaderUpstreamRequestID 上游报障编号的**改名回传名**（Stage 3）：网关命名空间唯一，
	// 报障编号无双义。只由 handler/v1 的 writeHeader 产生——上游自称该名一律丢弃，否则
	// 客户端分不清「网关改名的供应商号」与「上游自写值」。
	HeaderUpstreamRequestID = "X-Upstream-Request-Id"

	// HeaderVendorRequestID 上游报障编号的另一种拼法（canonical 形态 Request-Id）：
	// Anthropic 系用它，与 HeaderRequestID 一起归一到 HeaderUpstreamRequestID——客户端只需
	// 认一个输出名，向供应商报障时拿到的仍是原号。同一标识在响应体内写作 request_id
	// （service/anthropic 已按同一来源处理错误信封的该键），头体两处指向同一个供应商编号。
	HeaderVendorRequestID = "Request-Id"

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
