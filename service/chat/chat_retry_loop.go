package chat

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
	"github.com/qkf688/llmux/service/chatcore"
)

// retryLoopInput 描述在**单个候选池**上执行重试循环所需的全部输入。
// 真实模型路径只有一个候选池（来自 ProvidersWithMeta）；虚拟模型路径为每个
// 真实模型各构建一个候选池，逐个调用本循环。两条路径的差异全部收敛为本结构体
// 的字段，循环体内不含路径判断。
type retryLoopInput struct {
	Ctx context.Context
	// Start 是**请求级**开始时刻（handler startReq），整个循环每次尝试共用同一值：
	// 每条尝试日志的 ProxyTime 都是「请求进入网关至今」的累计耗时，含此前所有
	// 失败尝试的时长，非单次尝试耗时。与 FirstChunkTime 同零点（勿改 per-attempt）。
	Start   time.Time
	Style   string
	Before  Before
	ReqMeta models.ReqMeta
	IOLog   bool

	// Pool 候选池。Pool.WeightItems / Pool.PriorityItems 会被循环**就地修改**
	// （淘汰失败候选），调用方传入后不应再复用其内容做其它判断。
	Pool CandidatePool

	// RealModelName / Model 标识本候选池所属的真实模型：真实路径为请求模型本身，
	// 虚拟路径为当前正在尝试的 ordered model。
	RealModelName string
	Model         *models.Model

	MaxRetry      int
	ClientTimeout time.Duration

	// Deadline 超时信号：真实路径为本次请求的局部计时器，虚拟路径为跨真实模型
	// 共享的全局计时器，故由调用方持有并传入通道而非在此创建。
	Deadline <-chan time.Time
	// DeadlineErr 为 Deadline 触发时返回的错误；两条路径语义不同（单模型重试超时
	// vs 虚拟模型全局超时），由调用方给定。
	DeadlineErr error

	// LogAttrs 追加到本循环所有选路日志的上下文键值（虚拟路径带
	// virtual_model / real_model，便于在多真实模型间区分）。
	LogAttrs []any
}

type retryLoopStatus int

const (
	// retryLoopSucceeded 某个候选转发成功，Result 有效。
	retryLoopSucceeded retryLoopStatus = iota
	// retryLoopExhausted 候选池穷尽：已无可选候选，或 MaxRetry 用尽。
	// 语义由调用方决定——真实路径视为整体失败，虚拟路径换下一个真实模型。
	retryLoopExhausted
	// retryLoopAborted ctx 取消、超时，或遇到不可重试错误，须直接终止整个请求。
	retryLoopAborted
)

type retryLoopOutcome struct {
	Status retryLoopStatus
	// Result 仅在 Status == retryLoopSucceeded 时有效。
	Result *BalanceResult
	// Err 仅在 Status == retryLoopAborted 时有效。
	Err error
}

// runProviderRetryLoop 在给定候选池上执行「选路 → 尝试 → 淘汰」重试循环。
//
// retryLog 通道的生命周期完全由本函数持有：一个候选池对应一条通道，循环退出即
// 关闭。调用方无需（也不应）手动 close——虚拟路径此前在 5 个返回分支各写一次
// close，漏一处即泄漏 RecordRetryLog goroutine。
//
// RecordRetryLog 经 bgtask 登记：其 ctx 不随请求取消（写库/权重衰减必须跑完，见
// bgtask 包注释），且进程关闭时会等它把通道剩余元素排空落库。
func runProviderRetryLoop(in retryLoopInput) retryLoopOutcome {
	retryLog := make(chan models.ChatLog, in.MaxRetry)
	defer close(retryLog)
	modelWithProviderMap := in.Pool.ModelWithProviderMap
	bgtask.Go(func(ctx context.Context) { RecordRetryLog(ctx, retryLog, modelWithProviderMap) })

	// style 已由 handler 层固定路由校验，必可解析（与 attempt 内同容忍度）。
	// 循环不变量，提到循环外只算一次。
	clientWire, _ := consts.WireFormatOfStyle(consts.Style(in.Style))

	for retry := range in.MaxRetry {
		select {
		case <-in.Ctx.Done():
			return retryLoopOutcome{Status: retryLoopAborted, Err: in.Ctx.Err()}
		case <-in.Deadline:
			return retryLoopOutcome{Status: retryLoopAborted, Err: in.DeadlineErr}
		default:
		}

		id, err := chatcore.SelectByPriorityAndWeight(in.Pool.WeightItems, in.Pool.PriorityItems)
		if err != nil {
			// 选不出候选说明池已空，属正常穷尽而非系统错误：调用方据此决定
			// 是整体失败还是换下一个真实模型。
			slog.Warn("no selectable provider left", retryLogAttrs(in.LogAttrs, "error", err)...)
			return retryLoopOutcome{Status: retryLoopExhausted}
		}

		modelWithProvider, ok := in.Pool.ModelWithProviderMap[*id]
		if !ok {
			delete(in.Pool.WeightItems, *id)
			continue
		}

		provider := in.Pool.ProviderMap[modelWithProvider.ProviderID]

		// 选路：端点 → 分组 → 凭据（service/channel 三层）。任何一层失败只淘汰该候选
		// （与 providers.New 失败同语义），池中其余候选仍应尝试；凭据级/组织级的
		// 分层处置（组内换 key、冷却）在下面内层循环，此处不区分 sentinel。
		// probedThisCandidate：#6-3 每候选每请求最多探活一次，防恢复后再次耗尽死循环。
		probedThisCandidate := false
		snapshot, err := channel.NewAssembler(repos()).Assemble(in.Ctx, provider)
		if err != nil {
			slog.Error("failed to assemble channel snapshot", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
			delete(in.Pool.WeightItems, *id)
			continue
		}
		// 进程级共享选路器：轮询状态（分组档内 / 凭据组内）跨请求推进；新建实例
		// 会让指针恒从 0 起算、多凭据/多组退化为确定性首选（见 channel.Select 注释）。
		selection, err := channel.DefaultSelector().Select(snapshot, clientWire, in.RealModelName, time.Now())
		if err != nil {
			// 凭据耗尽：惰性探活 temp_unsched（#6-3）。成功则刷新快照重选；
			// 端点/分组不可用不探（非凭据层问题）。Select 在凭据失败时已带回
			// Endpoint/Group，探活锁定该组——禁止再 SelectGroup（二次推进 RR）。
			if errors.Is(err, channel.ErrNoCredentialAvailable) && !probedThisCandidate && selection.Group.ID != 0 {
				probedThisCandidate = true
				if probeRecoverInGroup(in.Ctx, provider, selection.Endpoint, snapshot.CredentialsByGroup[selection.Group.ID]) {
					refreshed, refreshErr := channel.NewAssembler(repos()).Assemble(in.Ctx, provider)
					if refreshErr == nil {
						if sel2, selErr := channel.DefaultSelector().Select(refreshed, clientWire, in.RealModelName, time.Now()); selErr == nil {
							selection = sel2
							snapshot = refreshed
							err = nil
						}
					}
				}
			}
			if err != nil {
				// Warn 而非 Error：Select 失败含「全部凭据冷却」这类组内故障转移的
				// **正常产出**（前一轮刚把 key 打冷却），配上 ErrEndpointUnavailable/
				// ErrNoGroupMatches 等配置态，均非系统异常；按候选淘汰继续即可。
				slog.Warn("no channel selection", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
				delete(in.Pool.WeightItems, *id)
				continue
			}
		}
		// 防御性分支（不变量）：Select 成功则端点协议必在 protocolMetaTable 中——
		// SelectEndpoint 已按同一张表过滤未知协议端点，TypeOfProtocol 与之共享表不可能发散。
		providerType, ok := consts.TypeOfProtocol(consts.Protocol(selection.Endpoint.Protocol))
		if !ok {
			slog.Error("unknown endpoint protocol", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "protocol", selection.Endpoint.Protocol)...)
			delete(in.Pool.WeightItems, *id)
			continue
		}

		// 组内凭据预算（#13 组内故障转移）：每次凭据级失败写冷却剔除一条 key，
		// 组内最多尝试这么多轮后必无可用凭据——预算锁定上限收敛（不依赖冷却写库
		// 是否成功：写库失败时 Refresh 到的快照不变，预算用尽即退出）。
		assembler := channel.NewAssembler(repos())
		budget := len(snapshot.CredentialsByGroup[selection.Group.ID])
		if budget == 0 {
			budget = 1
		}

		// orgFailure 待组织级处置的最后一次凭据级失败。内层循环有两种组耗尽收敛方式：
		//  ① RetryCredential 返回 ErrNoCredentialAvailable（真耗尽，前提是冷却写库成功）
		//  ② 预算自然用尽（最后一次凭据级失败后换 key 成功但预算也耗尽——冷却写库
		//     失败时 ① 不会出现，预算用尽就是组耗尽的替代收敛信号）
		// 两种都必须统一在循环外做组织级处置：否则冷却写库失败时该候选的组织级
		// ConsecutiveFailures/衰减静默停摆，恰在系统最需要保护的高故障期失效。
		var orgFailure *singleProviderAttemptResult

		for inner := 0; inner < budget; inner++ {
			// 内层换 key 重试同样受 ctx 取消 / 整体超时约束：取消后继续换 key 只会
			// 空转重试（冷却本无罪的 key）。
			select {
			case <-in.Ctx.Done():
				return retryLoopOutcome{Status: retryLoopAborted, Err: in.Ctx.Err()}
			case <-in.Deadline:
				return retryLoopOutcome{Status: retryLoopAborted, Err: in.DeadlineErr}
			default:
			}

			chatModel, err := providers.New(providerType, selection.Config, provider.Proxy)
			if err != nil {
				// 单个供应商实例化失败（配置损坏等）只淘汰该候选，池中其余候选仍应尝试。
				slog.Error("failed to create provider", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
				delete(in.Pool.WeightItems, *id)
				orgFailure = nil
				break
			}
			client := providers.GetClientWithProxy(in.ClientTimeout, chatModel.GetProxy())
			slog.Info("using provider", retryLogAttrs(in.LogAttrs,
				"provider", provider.Name,
				"model", modelWithProvider.ProviderModel,
				"proxy", chatModel.GetProxy(),
				"group", selection.Group.Name,
				"credential", credentialLabel(selection.Credential),
				"retry", retry+1,
				"inner", inner+1,
			)...)

			result := executeSingleProviderAttempt(singleProviderAttemptInput{
				Ctx:               in.Ctx,
				Start:             in.Start,
				Style:             in.Style,
				Before:            in.Before,
				RealModelName:     in.RealModelName,
				ReqMeta:           in.ReqMeta,
				IOLog:             in.IOLog,
				Retry:             retry + inner, // 换 key 也是独立上游尝试：计数如实反映第几次
				Provider:          provider,
				ModelWithProvider: modelWithProvider,
				Model:             in.Model,
				Selection:         selection,
				ChatModel:         chatModel,
				Client:            client,
			}, retryLog)

			if result.FatalErr != nil {
				return retryLoopOutcome{Status: retryLoopAborted, Err: result.FatalErr}
			}
			if result.Success {
				return retryLoopOutcome{Status: retryLoopSucceeded, Result: &BalanceResult{
					Response:     result.Response,
					LogID:        result.LogID,
					ProviderName: provider.Name,
					SideChannel:  result.SideChannel,
				}}
			}

			if !result.CredentialFailure {
				// 组织级失败：请求本身问题（其余 4xx / 流转换失败）——换 key 无意义，
				// 现有淘汰语义（attempt 内已累计组织级 adjustment）。
				applyProviderSelectionResult(in.Pool.WeightItems, in.Pool.PriorityItems, *id, result)
				orgFailure = nil
				break
			}

			// 凭据级失败：组内换 key。冷却已由 attempt 写库，刷新快照后锁组重选
			// （锁定原分组/原端点，channel.RetryCredential）——冷却中的 key 被剔除，
			// 轮询指针自然推进到下一条。
			refreshed, err := assembler.Assemble(in.Ctx, provider)
			if err != nil {
				// 刷新失败 = 无法再取候选，属基础设施异常：只淘汰不累计（与首次
				// Assemble / Select 失败同语义——凭据级失败累计滞后不在此类）。
				slog.Error("refresh channel snapshot after credential failure", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
				delete(in.Pool.WeightItems, *id)
				orgFailure = nil
				break
			}
			retrySelection, err := channel.DefaultSelector().RetryCredential(refreshed, selection.Group.ID, selection.Endpoint, time.Now())
			if err != nil {
				if errors.Is(err, channel.ErrNoCredentialAvailable) {
					// 真组耗尽：先惰性探活（#6-3）。成功则再锁组重选一次；
					// 仍失败才交由循环外组织级处置。
					if !probedThisCandidate {
						probedThisCandidate = true
						if probeRecoverInGroup(in.Ctx, provider, selection.Endpoint, refreshed.CredentialsByGroup[selection.Group.ID]) {
							if refreshed2, rerr := assembler.Assemble(in.Ctx, provider); rerr == nil {
								if sel2, serr := channel.DefaultSelector().RetryCredential(refreshed2, selection.Group.ID, selection.Endpoint, time.Now()); serr == nil {
									selection = sel2
									snapshot = refreshed2
									orgFailure = &result
									continue
								}
							}
						}
					}
					// 探活未恢复 / 已探过：交由循环外统一做组织级处置——按最后一次凭据级失败的
					// 分类淘汰候选（single key 组与现状分类语义等价）并累计
					// ConsecutiveFailures（凭据级失败本身不累计，设计定案第 5 节
					// 「层内耗尽才累计组织级」——两级语义不重叠）。
					slog.Warn("credentials exhausted in group", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "group", selection.Group.Name, "error", err)...)
					orgFailure = &result
					break
				}
				// 组被并发删除 / 解密失败：候选不可用而非「凭据耗尽」，只淘汰不累计
				//（配置变更与资源问题不该计入组织级故障）。
				slog.Warn("credential retry selection failed", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
				delete(in.Pool.WeightItems, *id)
				orgFailure = nil
				break
			}
			selection = retrySelection
			snapshot = refreshed
			orgFailure = &result // 预算继续消耗：下一次迭代若成功会 return，若失败覆盖本条
		}

		if orgFailure != nil {
			applyProviderSelectionResult(in.Pool.WeightItems, in.Pool.PriorityItems, *id, *orgFailure)
			applyProviderFailureAdjustments(in.Ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
		}
	}

	return retryLoopOutcome{Status: retryLoopExhausted}
}

// retryLogAttrs 复制基础属性后再追加，避免直接 append 复用底层数组时
// 多次调用互相覆盖已写入的键值。
func retryLogAttrs(base []any, extra ...any) []any {
	attrs := make([]any, 0, len(base)+len(extra))
	attrs = append(attrs, base...)
	return append(attrs, extra...)
}
