package service

// CacheControl 缓存控制配置 (Anthropic 特有功能)
// 参考: https://docs.anthropic.com/claude/docs/prompt-caching
// 参考: E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\cache_control.go
//
// 缓存控制可以应用于:
// - Message 级别: 缓存整个消息
// - Tool 级别: 缓存工具定义
// - ContentPart 级别: 缓存特定内容部分
//
// 使用场景:
// - 长系统提示词
// - 大量工具定义
// - 重复使用的上下文
//
// 成本优化:
// - 缓存的内容在后续请求中可以显著降低成本
// - 缓存有效期: 5 分钟
type CacheControl struct {
	// Type 缓存类型
	// 当前只支持 "ephemeral" (临时缓存)
	Type string `json:"type"`
}
