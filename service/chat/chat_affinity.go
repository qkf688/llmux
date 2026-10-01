package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/balancer"
	"github.com/qkf688/llmux/service/virtualmodel"
)

// chat 层缓存亲和编排单源：开关判定、钉定读取与成功改钉收口在此（retry loop 与虚拟
// 路径只调这几个入口，不各自解释设置/空键语义）。「粘性是选路偏好覆盖」——本文件只
// 回答「钉没钉、钉的是谁」，不触碰成败判定、冷却与重试语义（全链复用现状）。

// affinityKeyFor 返回本请求参与缓存亲和的键；设置关闭或键为空（未识别对话身份）
// 返回 ""——空键是粘性的统一「关闭」信号（balancer 表对空键 Put/Get 均视为未钉）。
func affinityKeyFor(ctx context.Context, before Before) string {
	if !getCacheAffinityEnabled(ctx) {
		return ""
	}
	return before.affinityKey
}

// stickyTargetFor 查询本对话的钉定目标；ok=false = 未钉（开关关闭/键空/闲置过期/
// LRU 淘汰均在此列），调用方回退常规选择即可，无需区分原因。
func stickyTargetFor(ctx context.Context, before Before) (balancer.Target, bool) {
	key := affinityKeyFor(ctx, before)
	if key == "" {
		return balancer.Target{}, false
	}
	return balancer.DefaultAffinity().Get(key, time.Now())
}

// recordStickyTarget 成功后改钉实际命中的目标（首次钉定与转移后改钉同一入口）：
// 目标永远是「这一次真成了」的组合，故粘性不会把对话钉死在失败目标上。开关关闭/
// 键空时无操作。并发同对话请求各自成功时后写覆盖——两个目标都可用，钉谁都无害。
func recordStickyTarget(ctx context.Context, before Before, target balancer.Target) {
	key := affinityKeyFor(ctx, before)
	if key == "" {
		return
	}
	balancer.DefaultAffinity().Put(key, target, time.Now())
}

// clearStickyTarget 清掉本对话指向 servingMWP 的钉定，返回是否真的清掉了。提交后
// 失败（上游腰斩等，见 chat_record.go 的触发注释）时调用：这次没能从钉定目标拿到
// 成功，且响应已开写、无法换家从头再来——不清钉会让该对话的下一个请求继续优先撞
// 它，形成「每请求必交一次失败」的循环（实测：某中转 92s 处腰斩，同一段对话下一轮
// 仍先撞它）。
//
// servingMWP = 本次**实际提供服务**的关联（含故障转移后的赢家）而非「读钉时看到的那
// 个」：转发成功那一刻 recordStickyTarget 已把钉子改指到它，故比对它才能既清得掉
// 常态转移、又不误删并发里另一次请求刚写进去的新钉（判据在 balancer.DeleteIf）。
// 0 表示无可比对对象（表内真实钉定的 ModelWithProviderID 恒非 0），比对必然落空。
//
// 只清本对话：同表其它对话的钉定原样存活。Reset 才是全清，会造成全体活跃会话集体
// 失忆，故不在此路径使用。留一行痕是因为自愈动作不可见 = 事后无从排查；行数与
// RecordLog「一次失败恰一条留痕」同口径，不与它的 ERROR 重复。
func clearStickyTarget(ctx context.Context, before Before, servingMWP uint) bool {
	key := affinityKeyFor(ctx, before)
	if key == "" {
		return false // 开关关闭或未识别对话身份：本就不粘，无钉可清
	}
	if !balancer.DefaultAffinity().DeleteIf(key, servingMWP) {
		return false
	}
	slog.Info("sticky target cleared", "affinity_key", key, "model_with_provider_id", servingMWP)
	return true
}

// preferRealModelsFirst 把钉定的真实模型稳定前移到尝试首位（其余模型相对顺序不变）。
// 在 chat 编排层做**本地副本**重排，不动 virtualmodel 包与策略状态——虚拟模型边界只做
// 虚拟→真实解析，粘性偏好是编排层的覆盖（策略的 UpdateRoundRobinIndex 等语义不变）。
// 未钉（pinnedModelID 为 0）或钉定模型不在映射内（映射被改/模型被删）→ 原切片返回。
//
// Ruling（2026-09-23 审查回批）：前移**不做 mapping Priority 分档**，与候选层
// 「粘性不越权优先级档」（chatcore.SelectByPriorityAndWeight）刻意不对称——本列表是
// 故障转移序列而非并行分摊策略，前移只改「先试哪个」、失败照常按原序转移，且钉定只
// 来自上一次真实成功。代价：管理员调高他人 mapping priority 后，活跃对话仍钉旧模型
// 直到闲置过期（粘性表 30min 空闲 TTL 收敛）；测试 TestPreferRealModelsFirst 锁定该语义。
func preferRealModelsFirst(models []virtualmodel.OrderedRealModel, pinnedModelID uint) []virtualmodel.OrderedRealModel {
	if pinnedModelID == 0 {
		return models
	}
	for i, m := range models {
		if m.Model.ID != pinnedModelID {
			continue
		}
		if i == 0 {
			return models
		}
		out := make([]virtualmodel.OrderedRealModel, 0, len(models))
		out = append(out, models[i])
		out = append(out, models[:i]...)
		out = append(out, models[i+1:]...)
		return out
	}
	return models
}
