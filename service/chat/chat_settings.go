package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
	"github.com/qkf688/llmux/service/transform"
)

// getStrictCapabilityMatch 读取严格能力匹配设置。
// 默认值与 setting schema 一致（true），避免与 handler/settings.GetStrictCapabilityMatch 反向默认导致行为分叉。
func getStrictCapabilityMatch(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyStrictCapabilityMatch, true)
}

// getCacheAffinityEnabled 读取缓存亲和（粘性选路）开关。
// 默认值与 setting schema 一致（true）：钉定失效无害退化为现状选择（chat_affinity.go），
// 默认开让提示词缓存省钱即刻生效。
func getCacheAffinityEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyCacheAffinityEnabled, true)
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

// getRawLogFieldMaxBytes 原始日志落库的单字段文本上限（设置单位 KiB，默认 256、
// 下限 16，Stage 2-D）。一个上限统一盖四字段（请求体/原始请求体/原始响应体/Error）：
// 体积闸（upstream_max_response_mib）管内存瞬时峰值，这里管 SQLite 体积——单行 MiB 级
// 大文本会拖慢日志页查询；截断按 UTF-8 rune 边界并带标记（common.TruncateUTF8 单源）。
func getRawLogFieldMaxBytes(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyRawLogFieldMaxKiB, 256, 16) << 10
}

// getResponseLogMaxBytes 回复类日志（ResponseBody/流式 ChatIO 累积）的单条上限
// （设置单位 KiB，默认 2048、下限 16、硬顶 16384，Stage 23）。与 getRawLogFieldMaxBytes
// 的差异：这是「看全文」口径——采集开关本就只在调试时打开，故默认给足原文体量，
// 超限存头尾两段而非纯尾切；两上限管的对象不同（采集四字段 vs 回复整条），不合并。
func getResponseLogMaxBytes(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyResponseLogMaxKiB, 2048, 16) << 10
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

// getRequestHeaderTimeout 获取「流式请求单次尝试等待上游响应头」的超时（秒）。
// 默认值与 setting schema 一致（30），语义 = 原 per-model TimeOut 的 ResponseHeaderTimeout 角色。
func getRequestHeaderTimeout(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyRequestHeaderTimeout, 30, 1)
}

// getNonStreamResponseTimeout 获取「非流式请求单次尝试等待完整回答」的窗口（秒）。
// 默认值与 setting schema 一致（90）：上游非流式请求要生成完整回答才发响应头，故该
// 窗口等价「整段生成预算」——沿用等头那个 30s 会把平均耗时接近 30s 的慢供应商长期
// 判死（实测 2.8% 稳定失败率，Stage 38 拆键的现场证据）。与总超时（600s 帽子）解耦：
// 窗口到点只消耗本次尝试，后续候选仍有完整窗口。
func getNonStreamResponseTimeout(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyNonStreamResponseTimeout, 90, 1)
}

// upstreamAttemptWindow 解析「单次上游尝试」实际施加的等待窗口（Stage 38 单源）：
// 按入站 stream 分流——流式等响应头、非流式等整段回答。真实/虚拟两条路径共用此函数，
// 窗口值随后既驱动 providers.GetClientWithProxy 的 ResponseHeaderTimeout，也随尝试
// 传到失败文案（印出的秒数与实际施加值同源，不在失败时重读设置——那时请求 ctx 可能
// 已死，读设置会退默认值印错数）。
func upstreamAttemptWindow(ctx context.Context, stream bool) time.Duration {
	if stream {
		return time.Second * time.Duration(getRequestHeaderTimeout(ctx))
	}
	return time.Second * time.Duration(getNonStreamResponseTimeout(ctx))
}

// conservativeRequestTotalTimeoutFallback 是 request_total_timeout 读失败时的保守值。
// 读失败 = DB 故障 / 存值解析失败 / 低于 Min 越界（models.GetSettingIntE 既有契约）。
//
// 为什么 60s：
//   - 默认 600s 的 1/10；读失败是异常态，按"宁可误杀慢请求也要保住网关存活"的取向收紧
//   - stage36 已补齐"读失败期间真实失败照记、恢复后结账"，慢请求即便被误杀也会按超时归因结算
//   - 不放 settings 表：避免与"默认 600"是两个不同语义的配置项混淆
//
// 为什么是代码常量而非 settings：
//   - 60s 是 stage37 的语义常量，不是用户面向的配置；混进 settings 会让用户误以为可以调
//   - 改这里要发版——这正是想要的"不让人随手调"的效果
const conservativeRequestTotalTimeoutFallback = 60 * time.Second

// getRequestTotalTimeout 获取整个请求的总时长上限（秒），含读体/预处理/选路/上游
// 调用/流式输出全程。默认值与 setting schema 一致（600：正常长回答不被截的余量）。
// Stage 33 起该值挂进请求 ctx 的 deadline（见 WithRequestTotalTimeout），
// 到点整个请求被掐断并按超时归因结算，不再是「仅阻止下一次重试」的计时器。
//
// 读失败处置（Stage 37）：DB 故障 / 解析失败 / Min 越界 → 返回 conservativeRequestTotalTimeoutFallback
// （60s）并通过 settings.LogReadSkip 留痕——按设置项限频（Stage 36）每 60 秒最多一条，
// 同一窗口内被压掉的次数在下一条里带出。
func getRequestTotalTimeout(ctx context.Context) int {
	value, err := settingsReader.IntE(ctx, models.SettingKeyRequestTotalTimeout, 600, 1)
	if err != nil {
		settings.LogReadSkip("request total timeout conservative fallback",
			models.SettingKeyRequestTotalTimeout, err)
		return int(conservativeRequestTotalTimeoutFallback / time.Second)
	}
	return value
}

// WithRequestTotalTimeout 给请求挂上网关总超时（Stage 33）：派生带 deadline 的请求 ctx，
// 到点整个请求（含流式输出）被掐断——「总超时」从此是真上限，不再只是「别再开下一次重试」。
// 超时（DeadlineExceeded）与调用方撤走（Canceled）在 ctx 上可区分，归因单源在
// chat_settlement.requestAbandoned。cancel 必须在 handler 返回（响应写完）时调用，
// 过早 cancel 会掐断正在输出的流。
func WithRequestTotalTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Second*time.Duration(getRequestTotalTimeout(ctx)))
}

// getStreamFirstByteTimeout 获取流式响应头后首字节等待超时（秒），
// 供本包提交门的首字节看门狗使用（chat_commit_gate / chat_stream_watchdog）。
// 默认值与 setting schema 一致（15）。
func getStreamFirstByteTimeout(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyStreamFirstByteTimeout, 15, 1)
}

// getRequestMaxRetry 获取单候选池最大尝试次数。
// 默认值与 setting schema 一致（3），替代原 per-model MaxRetry。
func getRequestMaxRetry(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyRequestMaxRetry, 3, 1)
}

// getUpstreamMaxResponseBytes 获取「上游最大响应体」并换算为字节（设置单位 MiB，
// 见 models.SettingKeyUpstreamMaxResponseMiB）。默认与 schema 一致（32 MiB）；
// Int 的 min 参数与 schema Min 同为 1 MiB——脏值（0/负数）读出来仍是防线内取值。
func getUpstreamMaxResponseBytes(ctx context.Context) int64 {
	return int64(settingsReader.Int(ctx, models.SettingKeyUpstreamMaxResponseMiB, 32, 1)) << 20
}

// getCredHealthCooldown429Sec 获取凭据 429 限流冷却窗口（秒）。
// 默认值与 setting schema 一致（60），驱动 applyCredentialCooldown 的 429 分窗。
// 读失败返回 err（Stage 33 显式降级）：窗口不可信时调用方必须跳过冷却写，不得兜底。
func getCredHealthCooldown429Sec(ctx context.Context) (int, error) {
	return settingsReader.IntE(ctx, models.SettingKeyCredHealthCooldown429Sec, 60, 1)
}

// getCredHealthCooldownServerSec 获取凭据服务端错误冷却窗口（秒，5xx/超时/网络）。
// 默认值与 setting schema 一致（60）。读失败处置同 getCredHealthCooldown429Sec。
func getCredHealthCooldownServerSec(ctx context.Context) (int, error) {
	return settingsReader.IntE(ctx, models.SettingKeyCredHealthCooldownServerSec, 60, 1)
}

// getCredHealthAuthFailThreshold 获取凭据连续鉴权失败（401/403）判停阈值。
// 连续失败达阈值后 Status 变为 temp_unsched（选路剔除；探活恢复为 #6-3，人工可经
// 凭据 CRUD 改回 active）。默认值与 setting schema 一致（3）。
// 读失败返回 err（Stage 33 显式降级）：判停是停用类动作，阈值不可信时只计数不判停。
func getCredHealthAuthFailThreshold(ctx context.Context) (int, error) {
	return settingsReader.IntE(ctx, models.SettingKeyCredHealthAuthFailThreshold, 3, 1)
}

// getCredHealthProbeIntervalSec 获取凭据惰性探活最小间隔（秒）。
// temp_unsched 凭据距 LastProbeAt 不足该间隔则跳过探活（#6-3 频控）。
// 默认值与 setting schema 一致（60）。
func getCredHealthProbeIntervalSec(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyCredHealthProbeIntervalSec, 60, 1)
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
