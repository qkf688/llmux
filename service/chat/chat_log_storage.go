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
// 这些行静默丢失（停在建行值 processing，审计与指标双双失真）。
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

// SaveChatLog 建一行 chat_logs，并计入仪表盘统计（请求数 / 真实模型调用数）。
//
// 计数与留痕是两件事，本函数是「两者都做」的默认路径；只留痕不计数走
// SaveChatLogNoStats（见那里的口径说明）。
func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	defer cancel()

	recordChatLogStats(writeCtx, &log)
	return createChatLog(writeCtx, log)
}

// SaveChatLogNoStats 只建一行 chat_logs，**不**计入仪表盘统计。
//
// 何时用：当这一行**不代表一次请求**时。候选淘汰留痕（service/chat 的
// recordUnsentCandidate）是「候选级事件」——一次请求可淘汰 N 个候选、落 N 行；
// 若这 N 行各计一次 reqs/calls，仪表盘的「总请求数」与「真实模型调用排行」就按
// 候选数放大（选路淘汰在改动前是零行零计数，放大面是本次新增的）。
//
// 为什么不给「同一请求的多次淘汰」做去重而直接不计：去重要引入 request_id 级
// 的计数状态（新表/新状态机），而「淘汰行不是请求」这个事实本身就是判据——
// 不计数即可，无需再判一次「这次请求是否已被计过」。
//
// 注意与「请求数 = 建行数」的既有口径的关系：尝试行（executeSingleProviderAttempt
// 的建行）仍照常计数，故「一次请求 = 一次 reqs」在有尝试行时仍然成立；而「全部候选
// 都在选路阶段被淘汰」的请求**没有任何尝试行**，其 reqs 仍为 0——与本改动前一致
// （那类淘汰此前也是零行零计数），本改动只保证不因「多行淘汰」把请求数放大。
func SaveChatLogNoStats(ctx context.Context, log models.ChatLog) (uint, error) {
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	defer cancel()

	return createChatLog(writeCtx, log)
}

// recordChatLogStats 把「一行 chat_logs」计入仪表盘的两项计数。
// 与建行解耦的原因见 SaveChatLogNoStats。
func recordChatLogStats(ctx context.Context, log *models.ChatLog) {
	// 统计应独立于日志存储：即使关闭日志记录，也要更新仪表盘统计。
	if err := chatstats.RecordRequestStats(ctx, time.Now(), log.Name); err != nil {
		slog.Warn("failed to record request stats", "error", err)
	}
	// 真实模型统计：优先用 RealModelName，异常路径回退到 Name。
	realModelName := realModelStatKey(log)
	if err := chatstats.RecordRealModelRequestStats(ctx, realModelName); err != nil {
		slog.Warn("failed to record real model request stats", "error", err)
	}
}

// createChatLog 落库一行 chat_logs（完全关闭日志记录时跳过并返回 0）。
func createChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	// 检查是否完全关闭日志记录
	if getDisableAllLogs(ctx) {
		return 0, nil // 返回0表示不记录日志
	}

	if err := repos().ChatLog.Create(ctx, &log); err != nil {
		return 0, err
	}
	// 异步执行日志清理，避免阻塞主流程；经 bgtask 登记以便进程关闭时排空。
	bgtask.Go(cleanupLogsIfNeeded)
	return log.ID, nil
}

// preProviderFailureLog 是一次「上游请求从未被发出就失败」的请求日志字段快照。
// 五类同类出口（虚拟选模失败 / 模型不存在 / 虚拟跳过某真实模型 / 真实模型无启用
// 供应商 / 选路阶段候选被淘汰）共用同一拼装——字段口径只在这里定义一次，新增此类
// 出口只补一行调用；将来要补字段（如 UserAgent/RemoteIP）也只改这里，不会再出现
// 「同类失败行字段不一致」而需要逐点核对。（`executeSingleProviderAttempt` 的尝试建行是
// **已选出供应商并发出过请求**的另一类，字段口径不同，刻意不并入本结构。）
type preProviderFailureLog struct {
	// RequestedModel 调用方请求的模型名：虚拟路径为虚拟模型名，真实路径为真实模型名。
	RequestedModel string
	// RealModelName 已解析到的真实模型名；虚拟选模失败时尚未解析出任何真实模型，
	// 保持空串（此时日志只有请求模型名可归因）。
	RealModelName string
	// ProviderModel 仅「虚拟跳过某真实模型」与「候选淘汰」填：前者该真实模型此时即是
	// 候选上游模型名，后者是被淘汰关联的上游模型名；其余出口尚未选到供应商，留空。
	ProviderModel string
	// ProviderName / KeyGroupName / EndpointProtocol / EndpointURL 仅「候选淘汰」填：
	// 选路已经走完端点+分组两层、只卡在凭据层（或分组/端点层已定而凭据全不可用），
	// 这些是**已知且必须留名**的归因信息——不记就查不出「败在哪一家的哪一层」，
	// 而本次改动的目的正是让它在管理端可见。其余四类出口尚未选到，留空。
	ProviderName     string
	KeyGroupName     string
	EndpointProtocol string
	EndpointURL      string
	// ProxyTime 本行落库时刻「请求进入网关至今」的累计耗时。仅「候选淘汰」传（选路
	// 失败也真实消耗了请求时间，不记则这一列恒为 0，看不出这次请求白等了多久）；
	// 其余四类出口尚未进入候选循环，留零值。
	// 注意：本字段随 Create 直写，不受 updateChatLogByID 的 struct Updates 零值跳过影响。
	ProxyTime time.Duration
	// NoStats 跳过仪表盘计数（请求数 / 真实模型调用数）。
	//
	// **仅「候选淘汰」一类置 true**：那类行是**候选级**事件——一次请求可淘汰 N 个
	// 候选、落 N 行，若各计一次 reqs/calls，仪表盘的请求数就按候选数放大。
	// 其余四类每次请求至多落一行，计数语义仍成立，保持原样。
	//
	// 判据是「这行代表不了一次请求」，不是「这个状态要不要计数」——状态是 error
	// 的行照样要计数（如客户端侧失败出口那行），状态是 unsent 的行也可能要计数
	// （如模型不存在那次请求只落一行 unsent，它确实是那一次请求）。
	NoStats bool
	Error   string
}

// savePreProviderFailureLog 落一条「上游请求从未被发出就失败」的日志。
//
// 状态一律 `unsent`（**不是** `error`）：本函数的全部出口都没有发出任何一次上游请求，
// 与 error（上游故障/协议错误/体积闸等）是两回事。写在这里单点决定，调用方不再
// 各自挑状态——避免「候选淘汰被当成供应商失败」这类假归因从调用点长出来。
// 直写终态、不经 processing：本结构产出的是终态行，不存在「在途」中间时刻。
//
// 写库失败只记 slog、不改变调用方的返回错误：请求本已注定失败，日志是副作用，
// 不该把失败原因从「模型不存在」偷换成「数据库错误」（对外同为 500 通用文案，
// 但服务端日志里这两者的归因价值完全不同）。
func savePreProviderFailureLog(ctx context.Context, reqMeta models.ReqMeta, style string, f preProviderFailureLog) {
	row := models.ChatLog{
		Name:             f.RequestedModel,
		RealModelName:    f.RealModelName,
		ProviderModel:    f.ProviderModel,
		ProviderName:     f.ProviderName,
		KeyGroupName:     f.KeyGroupName,
		EndpointProtocol: f.EndpointProtocol,
		EndpointURL:      f.EndpointURL,
		Status:           models.ChatLogStatusUnsent,
		Style:            style,
		Error:            f.Error,
		ProxyTime:        f.ProxyTime,
		ApiKeyID:         reqMeta.ApiKeyID,
		ApiKeyName:       reqMeta.ApiKeyName,
		RequestID:        reqMeta.RequestID,
	}
	// 计数与留痕分流：候选淘汰行是候选级事件，同一请求可能落多行，计进去会把
	// 仪表盘的请求数按候选数放大（口径见 SaveChatLogNoStats）。
	var err error
	if f.NoStats {
		_, err = SaveChatLogNoStats(ctx, row)
	} else {
		_, err = SaveChatLog(ctx, row)
	}
	if err != nil {
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
