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

// chatLogDetachedCtx 派生「日志路径」用 ctx（Stage 17 + 审查回批，派生表达式单源）：
// WithoutCancel 保留 ctx 内值/trace 语义、只摘取消信号，独立短超时兜底挂死。
// 消费方：SaveChatLog / updateChatLogByID / persistChatLog / maybeClearRawOnSuccess
// 的写库与清理，以及失败分支的设置读取（withDetachedLogCtx）。
func chatLogDetachedCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), chatLogWriteTimeout)
}

// withDetachedLogCtx 在不陪葬请求 ctx 的派生上下文里同步执行 fn（审查回批 W2）：
// 日志路径上的设置读取若挂死 ctx，GORM 读失败会静默退默认值——管理员配置的
// 采集上限恰在失败场景失效，与写库侧收口口径分裂。fn 返回即 cancel，
// 派生 ctx 不得逃逸出 fn（需要逃逸的场景直接用 chatLogDetachedCtx 自管 cancel）。
func withDetachedLogCtx(ctx context.Context, fn func(ctx context.Context)) {
	detached, cancel := chatLogDetachedCtx(ctx)
	defer cancel()
	fn(detached)
}

// realModelStatKeyOf 真实模型维度统计的键：优先真实命中模型名，为空时回退
// 用户请求的模型名（虚拟选模失败的异常路径只写得出后者）。
//
// 口径单源的理由：calls 在建行时刻（SaveChatLog）记一次，token 在结算时刻
// （RecordLog）再记一次，两次写同一张表的同一主键。口径若在两处各写一遍，
// 将来改动只落一处就会让同一模型裂成两行各自计数——且这种错账在 UI 上完全
// 看不出来（排行照常显示，只是数字对不上）。
//
// 两个参数分开传而不是收 *models.ChatLog：结算时刻的 processer 返回值只填了
// 耗时与 usage，模型名不在其中（见 process.go 的 createProcesser），只能由
// 调用方按 RecordLogInput 的同源字段传进来。
func realModelStatKeyOf(realModelName, name string) string {
	if realModelName != "" {
		return realModelName
	}
	return name
}

// realModelStatKey 建行时刻的键，直接取日志快照上的两个字段。
func realModelStatKey(log *models.ChatLog) string {
	return realModelStatKeyOf(log.RealModelName, log.Name)
}

func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	// Stage 17 收口：统计与建行的写库 ctx 不随请求取消——日志留痕是审计副作用，
	// 不陪葬在途请求的生死。
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	defer cancel()

	// 统计应独立于日志存储：即使关闭日志记录，也要更新仪表盘统计。
	if err := chatstats.RecordRequestStats(writeCtx, time.Now(), log.Name); err != nil {
		slog.Warn("failed to record request stats", "error", err)
	}
	// 真实模型统计：优先用 RealModelName，异常路径回退到 Name。
	realModelName := realModelStatKey(&log)
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
