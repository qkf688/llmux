package chat

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform"
)

// getStrictCapabilityMatch 读取严格能力匹配设置。
// 默认值与 setting schema 一致（true），避免与 handler/settings.GetStrictCapabilityMatch 反向默认导致行为分叉。
func getStrictCapabilityMatch(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyStrictCapabilityMatch, true)
}

// getLogRawRequestResponse 获取原始请求响应记录选项
func getLogRawRequestResponse(ctx context.Context) models.RawLogOptions {
	value := settingsReader.String(ctx, models.SettingKeyLogRawRequestResponse, "")
	if value == "" {
		return models.RawLogOptions{} // 默认全部关闭
	}

	var options models.RawLogOptions
	if err := json.Unmarshal([]byte(value), &options); err != nil {
		return models.RawLogOptions{} // 解析失败，默认全部关闭
	}
	return options
}

func getLogRawRequestResponseErrorsOnly(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyLogRawRequestResponseErrorsOnly, false)
}

// getDisableAllLogs 获取是否完全关闭日志记录
func getDisableAllLogs(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyDisableAllLogs, false)
}

// getDisablePerformanceTracking 获取是否关闭性能追踪
func getDisablePerformanceTracking(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyDisablePerformanceTracking, false)
}

// getDisableTokenCounting 获取是否关闭 token 统计
func getDisableTokenCounting(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyDisableTokenCounting, false)
}

// getEnableRequestTrace 获取是否启用请求追踪
func getEnableRequestTrace(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyEnableRequestTrace, true)
}

// GetStripResponseHeaders 获取是否移除不必要的响应头（导出供 handler 使用）
func GetStripResponseHeaders(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyStripResponseHeaders, false)
}

// getEnableFormatConversion 获取是否启用格式转换
func getEnableFormatConversion(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyEnableFormatConversion, true)
}

// getRequestHeaderTimeout 获取单次尝试等待响应头的超时（秒）。
// 默认值与 setting schema 一致（30），语义 = 原 per-model TimeOut 的 ResponseHeaderTimeout 角色。
// 驱动 providers.GetClientWithProxy 的 ResponseHeaderTimeout。
func getRequestHeaderTimeout(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyRequestHeaderTimeout, 30, 1)
}

// getRequestTotalTimeout 获取整个请求的总预算超时（秒），含所有重试/故障转移。
// 默认值与 setting schema 一致（90 = 30×3，保证等头超时后后续候选仍有完整窗口）。
// 驱动 chat_balance 的重试循环 Deadline。
func getRequestTotalTimeout(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyRequestTotalTimeout, 90, 1)
}

// GetStreamFirstByteTimeout 获取流式响应头后首字节等待超时（秒），
// 导出供 handler/v1 首字节看门狗使用。
// 默认值与 setting schema 一致（15）。
func GetStreamFirstByteTimeout(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyStreamFirstByteTimeout, 15, 1)
}

// getRequestMaxRetry 获取单候选池最大尝试次数。
// 默认值与 setting schema 一致（3），替代原 per-model MaxRetry。
func getRequestMaxRetry(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyRequestMaxRetry, 3, 1)
}

// getCredHealthCooldown429Sec 获取凭据 429 限流冷却窗口（秒）。
// 默认值与 setting schema 一致（60），驱动 applyCredentialCooldown 的 429 分窗。
func getCredHealthCooldown429Sec(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyCredHealthCooldown429Sec, 60, 1)
}

// getCredHealthCooldownServerSec 获取凭据服务端错误冷却窗口（秒，5xx/超时/网络；
// 鉴权失败过渡期同窗，#6-2 判停落地后 401/403 不再走冷却）。
// 默认值与 setting schema 一致（60）。
func getCredHealthCooldownServerSec(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyCredHealthCooldownServerSec, 60, 1)
}

// buildThinkingClampConfig 从 ctx 读取设置 + model/association 白名单，构建思考档位钳制配置。
// 调用方在 supportsThinking=true 时调用（false 时 thinking 已被 stripThinkingFields 剥离，无需钳制）。
// autoFallback 复用 SettingKeyReasoningEffortDefaultValue（P0-2：设置键保留真实消费者）。
// unknownStrategy 复用 SettingKeyReasoningEffortUnknownStrategy。
// autoFallback 兜底校验：设置写入时已校验 enum，但绕过 API 的数据库脏值仍可能传入非法值，
// 此处再校验一次，非法则回退 "low"（ClampReasoningEffort 是纯函数，不自身校验）。
func buildThinkingClampConfig(ctx context.Context, model *models.Model, mwp *models.ModelWithProvider) *transform.ThinkingClampConfig {
	levels := mwp.ThinkingLevelsResolved(model)
	autoFallback := settingsReader.String(ctx, models.SettingKeyReasoningEffortDefaultValue, "low")
	if !models.IsSixLevelEffort(autoFallback) {
		slog.Warn("invalid reasoning_effort_default_value in settings, falling back to low",
			"got", autoFallback)
		autoFallback = "low"
	}
	unknownStrategy := settingsReader.String(ctx, models.SettingKeyReasoningEffortUnknownStrategy, "clamp_to_default")
	return &transform.ThinkingClampConfig{
		Levels:          levels,
		AutoFallback:    autoFallback,
		UnknownStrategy: unknownStrategy,
	}
}
