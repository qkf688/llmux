package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/balancer"
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

	// MaxRetry 本候选池内最多尝试几次（含组内换 key）。
	MaxRetry int
	// UpstreamWindow 单次尝试施加到 transport 的等待窗口（值 + 类别，流式等响应头 /
	// 非流式等整段回答，Stage 38 由 resolveUpstreamWindow 单源产出）。注意它不是
	// http.Client.Timeout（后者恒 0，见 providers/cache.go），而是 ResponseHeaderTimeout
	// 那个值；同一个值也随尝试传给失败文案，印出的秒数与实际施加值同源。
	UpstreamWindow upstreamWindow

	// 总时长上限不在本循环内维护（Stage 33）：由调用方挂在请求 ctx 的 deadline 上
	// （handler 入口 WithRequestTotalTimeout），Ctx.Done 一并覆盖「调用方撤走」与
	// 「网关掐断」，两者的归因区分在 chat_settlement.requestAbandoned。

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
// 关闭（含丢弃分支当场关闭）。调用方无需（也不应）手动 close——虚拟路径此前在
// 5 个返回分支各写一次 close，漏一处即泄漏 RecordRetryLog goroutine。
//
// RecordRetryLog 经 bgtask 登记：其 ctx 不随请求取消（留痕必须跑完，见 bgtask 包注释），
// 且进程关闭时会等它把通道剩余元素排空落库。登记被丢弃（关机收尾段的丢弃窗口）时
// 当场收手不再重试：没有消费者的通道投满即挂死，而此时重试结果也无处交付。
// 结算（权重/优先级衰减）不在该 goroutine 内做——它已移回产出侧经 beginFailureSettlement 判定（Stage 24）。
func runProviderRetryLoop(in retryLoopInput) retryLoopOutcome {
	retryLog := make(chan models.ChatLog, in.MaxRetry)
	if !bgtask.Go(func(ctx context.Context) { RecordRetryLog(ctx, retryLog) }) {
		// 丢弃窗口（Stage 35）：日志落库任务被丢，retryLog 无人消费——关通道防悬垂，
		// 并终止整个重试循环（进程正在收尾，重试结果无处交付；继续投递还会把日志
		// 堆进没人读的通道，缓冲一满 send 即挂死）。
		close(retryLog)
		return retryLoopOutcome{Status: retryLoopAborted, Err: errRetryLogTaskDiscarded}
	}
	defer close(retryLog)

	// style 已由 handler 层固定路由校验，必可解析（与 attempt 内同容忍度）。
	// 循环不变量，提到循环外只算一次。
	clientWire, _ := consts.WireFormatOfStyle(consts.Style(in.Style))

	// 缓存亲和（粘性是选路偏好覆盖）：钉定目标只取一次，候选层传 preferID（优先级
	// 档内钉定）、channel 层传 Prefer（分组/凭据）；未钉时全零值 = 现状选择。
	// 本次尝试无论换 key 还是换供应商，成功后都按实际命中改钉（下方 result.Success）。
	// Prefer 只对**钉定候选本身**生效（元组语义：分组/凭据偏好不脱离关联层独立生效，
	// 防止跨候选误钉同名 ID）；候选经池过滤/淘汰后 preferID 不命中即自然失效。
	sticky, hasSticky := stickyTargetFor(in.Ctx, in.Before)
	preferMWP := uint(0)
	if hasSticky {
		preferMWP = sticky.ModelWithProviderID
	}

	for retry := range in.MaxRetry {
		select {
		case <-in.Ctx.Done():
			return retryLoopOutcome{Status: retryLoopAborted, Err: in.Ctx.Err()}
		default:
		}

		id, err := chatcore.SelectByPriorityAndWeight(in.Pool.WeightItems, in.Pool.PriorityItems, preferMWP)
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

		// 分组白名单的命名空间 = **上游模型名**（modelWithProvider.ProviderModel），与写侧同源：
		// modelsync 按上游 /models 返回的模型 ID 填 KeyGroup.Models，autoassoc 目录校验、
		// testapi 诊断（GroupCandidatesForModel(mp.ProviderModel)）也都用上游名。传网关侧
		// in.RealModelName 会让重命名关联（ProviderModel ≠ Model.Name）在名单非空后恒
		// no group matches（历史缺口，2026-09-26 收口）。下面两处 Select 共用本变量——
		// 只改一处会让探活重选分支退回旧口径（有用例分别锁住两处）。
		upstreamModel := modelWithProvider.ProviderModel

		// 钉定偏好只随钉定候选走（元组语义，见上方 sticky 注释）。
		iterPrefer := channel.Prefer{}
		if hasSticky && *id == sticky.ModelWithProviderID {
			iterPrefer = channel.Prefer{GroupID: sticky.GroupID, CredentialID: sticky.CredentialID}
		}

		// 选路：端点 → 分组 → 凭据（service/channel 三层）。任何一层失败只淘汰该候选
		// （与 providers.New 失败同语义），池中其余候选仍应尝试；凭据级/组织级的
		// 分层处置（组内换 key、冷却）在下面内层循环，此处不区分 sentinel。
		// probedThisCandidate：#6-3 每候选每请求最多探活一次，防恢复后再次耗尽死循环。
		probedThisCandidate := false
		snapshot, err := channel.NewAssembler(repos()).Assemble(in.Ctx, provider, &modelWithProvider)
		if err != nil {
			slog.Error("failed to assemble channel snapshot", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
			recordUnsentCandidate(in, provider, modelWithProvider, channel.SelectionResult{}, err)
			delete(in.Pool.WeightItems, *id)
			continue
		}
		// 进程级共享选路器：轮询状态（分组档内 / 凭据组内）跨请求推进；新建实例
		// 会让指针恒从 0 起算、多凭据/多组退化为确定性首选（见 channel.Select 注释）。
		selection, err := channel.DefaultSelector().Select(snapshot, clientWire, upstreamModel, time.Now(), iterPrefer)
		if err != nil {
			// 凭据耗尽：惰性探活 temp_unsched（#6-3）。成功则刷新快照重选；
			// 端点/分组不可用不探（非凭据层问题）。Select 在凭据失败时已带回
			// Endpoint/Group，探活锁定该组——禁止再 SelectGroup（二次推进 RR）。
			if errors.Is(err, channel.ErrNoCredentialAvailable) && !probedThisCandidate && selection.Group.ID != 0 {
				probedThisCandidate = true
				if probeRecoverInGroup(in.Ctx, provider, selection.Endpoint, snapshot.CredentialsByGroup[selection.Group.ID]) {
					refreshed, refreshErr := channel.NewAssembler(repos()).Assemble(in.Ctx, provider, &modelWithProvider)
					if refreshErr == nil {
						if sel2, selErr := channel.DefaultSelector().Select(refreshed, clientWire, upstreamModel, time.Now(), iterPrefer); selErr == nil {
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
				// selection 带回落点：凭据层失败时端点与分组已定（Select 刻意回传，
				// 见 channel.Select 注释），归因因此能到「哪家的哪个分组」。
				recordUnsentCandidate(in, provider, modelWithProvider, selection, err)
				delete(in.Pool.WeightItems, *id)
				continue
			}
		}
		// 防御性分支（不变量）：Select 成功则端点协议必在 protocolMetaTable 中——
		// SelectEndpoint 已按同一张表过滤未知协议端点，TypeOfProtocol 与之共享表不可能发散。
		providerType, ok := consts.TypeOfProtocol(consts.Protocol(selection.Endpoint.Protocol))
		if !ok {
			slog.Error("unknown endpoint protocol", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "protocol", selection.Endpoint.Protocol)...)
			recordUnsentCandidate(in, provider, modelWithProvider, selection, fmt.Errorf("unknown endpoint protocol %q", selection.Endpoint.Protocol))
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
			default:
			}

			chatModel, err := providers.New(providerType, selection.Config, provider.Proxy)
			if err != nil {
				// 单个供应商实例化失败（配置损坏等）只淘汰该候选，池中其余候选仍应尝试。
				slog.Error("failed to create provider", retryLogAttrs(in.LogAttrs, "provider", provider.Name, "error", err)...)
				recordUnsentCandidate(in, provider, modelWithProvider, selection, err)
				delete(in.Pool.WeightItems, *id)
				orgFailure = nil
				break
			}
			client := providers.GetClientWithProxy(in.UpstreamWindow.Window, chatModel.GetProxy())
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
				UpstreamWindow:    in.UpstreamWindow,
			}, retryLog)

			if result.FatalErr != nil {
				return retryLoopOutcome{Status: retryLoopAborted, Err: result.FatalErr}
			}
			if result.Success {
				// 成功改钉：目标永远是「这一次真成了」的组合（含转移后的 key/供应商），
				// 粘性不会把对话锁死在失败目标上（AC-2）。Model 可缺省（直构
				// retryLoopInput 的调用方）：Target.ModelID 0 = 真实模型层不钉（Target
				// 契约），其余层照钉。
				modelID := uint(0)
				if in.Model != nil {
					modelID = in.Model.ID
				}
				recordStickyTarget(in.Ctx, in.Before, balancer.Target{
					ModelID:             modelID,
					ModelWithProviderID: modelWithProvider.ID,
					GroupID:             selection.Group.ID,
					CredentialID:        selection.Credential.ID,
				})
				return retryLoopOutcome{Status: retryLoopSucceeded, Result: &BalanceResult{
					Response:     result.Response,
					LogID:        result.LogID,
					ProviderName: provider.Name,
					SideChannel:  result.SideChannel,
					// 本次**实际服务**的关联（含故障转移后的赢家），不是「读钉时看到的
					// 那个」：紧邻上方的 recordStickyTarget 刚把钉子改指到它，故提交后
					// 失败时按它比对即可清掉——早期版本拿循环开头的旧 sticky 比对，会把
					// 「转移成功后腰斩」这个最常见的循环挡在门外。组内换 key 不影响本
					// 字段：清的是关联级记忆。
					ServingModelWithProviderID: modelWithProvider.ID,
					RealModelName:              in.RealModelName,
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
			refreshed, err := assembler.Assemble(in.Ctx, provider, &modelWithProvider)
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
							if refreshed2, rerr := assembler.Assemble(in.Ctx, provider, &modelWithProvider); rerr == nil {
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
			// 组内耗尽才消费的组织级结算同样走结算口径（Stage 24）：调用方已放弃时
			// 本次失败不是候选的故障，不结算；超时场景则改走不陪葬请求 ctx 的写库。
			settlement := beginFailureSettlement(in.Ctx)
			if settlement.Active {
				applyProviderFailureAdjustments(settlement.Ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
			}
			settlement.Close()
		}
	}

	return retryLoopOutcome{Status: retryLoopExhausted}
}

// recordUnsentCandidate 落一条「该候选在发出任何一次上游请求之前就被淘汰」的留痕行。
//
// 为什么必须由本函数写：chat_logs 的尝试行只在 executeSingleProviderAttempt 内建
// （即**已选出凭据、准备发请求**之后）。走到本函数的四个外层淘汰点从未进入尝试层，
// 于是这次淘汰在管理端是零痕迹——「这次请求为什么没走这家」只能去服务器控制台翻。
// 状态由建行单源定为 unsent（不是 error），供应商没做错任何事，不进「最近 10 次成败」。
//
// 刻意不覆盖内层凭据耗尽分支（credentials exhausted in group）：那条路径此前必已
// 产出尝试行，再补一行会把同一次上游尝试计成两行，双计请求数与模型调用数。
//
// sel 为零值时（端点层/分组层就已失败，Assemble 失败更是无任何选择结果）只留供应商
// 与模型名——归因到「已知的最深一层」，不臆造下层信息。
func recordUnsentCandidate(in retryLoopInput, provider models.Provider, mwp models.ModelWithProvider, sel channel.SelectionResult, cause error) {
	f := preProviderFailureLog{
		RequestedModel: in.Before.Model,
		RealModelName:  in.RealModelName,
		ProviderModel:  mwp.ProviderModel,
		ProviderName:   provider.Name,
		// 候选淘汰真实消耗了请求时间，零点与尝试行同源（in.Start 是请求级时刻）。
		// **这是淘汰时刻的中途累计值，不是该请求的最终耗时**：同一请求的多条淘汰
		// 行各不相同且都小于最终耗时（请求还会继续试下一个候选）。面板按此列排序
		// 不可当「请求耗时」读，只当「白等到被淘汰时过了多久」。
		// 「含探活的一次上游往返」只在惰性探活**失败**的分支成立——探活成功时走的是
		// 成功路径，不会到淘汰点。
		ProxyTime: time.Since(in.Start),
		Error:     cause.Error(),
		// 不计仪表盘统计：本行是**候选级**事件，一次请求可淘汰 N 个候选、落 N 行，
		// 各计一次会把「总请求数」与「真实模型调用排行」按候选数放大。
		NoStats: true,
	}
	if sel.Group.ID != 0 {
		f.KeyGroupName = sel.Group.Name
	}
	if sel.Endpoint.ID != 0 {
		f.EndpointProtocol = sel.Endpoint.Protocol
		// URL 独立解析，不取 sel.UpstreamURL：选路失败时 Select 不带它回来
		// （凭据层失败只回传 Endpoint/Group），靠它填会恒为空——那正是本次要答的
		// 「败在哪一家的哪个 URL」。解析只需端点与 provider config，不需要明文 key。
		if url, err := channel.ResolveUpstreamURL(provider, sel.Endpoint); err == nil {
			f.EndpointURL = url
		}
	}
	savePreProviderFailureLog(in.Ctx, in.ReqMeta, in.Style, f)
}

// retryLogAttrs 复制基础属性后再追加，避免直接 append 复用底层数组时
// 多次调用互相覆盖已写入的键值。
func retryLogAttrs(base []any, extra ...any) []any {
	attrs := make([]any, 0, len(base)+len(extra))
	attrs = append(attrs, base...)
	return append(attrs, extra...)
}
