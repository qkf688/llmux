package testsupport

// APIEnvelope 与 httpresp 统一响应体字段对齐，供 handler 集成测试解码。
type APIEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Data    T      `json:"data"`
}
