package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/chatstats"
)

// chatLogWriteTimeout chat_logs 全部写库点的独立写超时（Stage 17）：建行与
// 失败出口回写统一经 context.WithoutCancel + 本超时派生 ctx——最需要留痕的
// 失败（超时、客户端断连）恰恰发生在请求 ctx 已死的时刻，陪葬请求 ctx 会让
// 这些行静默丢失（停在建行占位的 success，审计与指标双双失真）。
const chatLogWriteTimeout = 5 * time.Second

func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	// Stage 17 收口：统计与建行的写库 ctx 从请求 ctx 派生但不随其取消
	//（WithoutCancel 保留 ctx 内的设置/trace 值语义）——日志留痕是审计副作用，
	// 不陪葬在途请求的生死。
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), chatLogWriteTimeout)
	defer cancel()

	// 统计应独立于日志存储：即使关闭日志记录，也要更新仪表盘统计。
	if err := chatstats.RecordRequestStats(writeCtx, time.Now(), log.Name); err != nil {
		slog.Warn("failed to record request stats", "error", err)
	}
	// 真实模型统计：优先用 RealModelName，异常路径回退到 Name。
	realModelName := log.RealModelName
	if realModelName == "" {
		realModelName = log.Name
	}
	if err := chatstats.RecordRealModelRequestStats(writeCtx, realModelName); err != nil {
		slog.Warn("failed to record real model request stats", "error", err)
	}

	// 检查是否完全关闭日志记录
	if getDisableAllLogs(writeCtx) {
		return 0, nil // 返回0表示不记录日志
	}

	if err := repos().ChatLog.Create(writeCtx, &log); err != nil {
		return 0, err
	}
	// 异步执行日志清理，避免阻塞主流程；经 bgtask 登记以便进程关闭时排空。
	bgtask.Go(cleanupLogsIfNeeded)
	return log.ID, nil
}

// preProviderFailureLog 是一次「尚未选出供应商就失败」的请求日志字段快照。
// 四类同类出口（虚拟选模失败 / 模型不存在 / 虚拟跳过某真实模型 / 真实模型无启用
// 供应商）共用同一拼装——字段口径只在这里定义一次，新增此类出口只补一行调用；
// 将来要补字段（如 UserAgent/RemoteIP）也只改这里，不会再出现「同类失败行字段
// 不一致」而需要逐点核对。（`executeSingleProviderAttempt` 的尝试建行是**已选出
// 供应商**的第另一类，字段口径不同，刻意不并入本结构。）
type preProviderFailureLog struct {
	// RequestedModel 调用方请求的模型名：虚拟路径为虚拟模型名，真实路径为真实模型名。
	RequestedModel string
	// RealModelName 已解析到的真实模型名；虚拟选模失败时尚未解析出任何真实模型，
	// 保持空串（此时日志只有请求模型名可归因）。
	RealModelName string
	// ProviderModel 仅「虚拟跳过某真实模型」填：该真实模型此时即是候选上游模型名；
	// 其余出口尚未选到供应商，留空。
	ProviderModel string
	Error         string
}

// savePreProviderFailureLog 落一条「未选出供应商即失败」的日志。
//
// 写库失败只记 slog、不改变调用方的返回错误：请求本已注定失败，日志是副作用，
// 不该把失败原因从「模型不存在」偷换成「数据库错误」（对外同为 500 通用文案，
// 但服务端日志里这两者的归因价值完全不同）。
func savePreProviderFailureLog(ctx context.Context, reqMeta models.ReqMeta, style string, f preProviderFailureLog) {
	if _, err := SaveChatLog(ctx, models.ChatLog{
		Name:          f.RequestedModel,
		RealModelName: f.RealModelName,
		ProviderModel: f.ProviderModel,
		Status:        "error",
		Style:         style,
		Error:         f.Error,
		ApiKeyID:      reqMeta.ApiKeyID,
		ApiKeyName:    reqMeta.ApiKeyName,
		RequestID:     reqMeta.RequestID,
	}); err != nil {
		// real_model 必须单独带上：RequestedModel 在虚拟路径是**虚拟模型名**，
		// 只打它则写库失败时看不出是哪个真实模型被跳过（原 saveVirtualSkipLog 打的就是它）。
		slog.Error("failed to save pre-provider failure log",
			"model", f.RequestedModel, "real_model", f.RealModelName, "error", err)
	}
}

// cleanupLogsIfNeeded 检查并清理超出保留条数的日志
func cleanupLogsIfNeeded(ctx context.Context) {
	// 获取日志保留条数设置
	retentionCount := getLogRetentionCount(ctx)
	if retentionCount <= 0 {
		return // 0 表示不限制
	}

	deleted, err := repos().ChatLog.EnforceRetention(ctx, retentionCount)
	if err != nil {
		slog.Error("failed to cleanup excess logs", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("auto cleaned up excess logs", "deleted", deleted, "retention", retentionCount)
	}
}

// getLogRetentionCount 获取日志保留条数设置
func getLogRetentionCount(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyLogRetentionCount, 0, 0)
}
