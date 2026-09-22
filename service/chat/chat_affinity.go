package chat

import (
	"context"
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

// preferRealModelsFirst 把钉定的真实模型稳定前移到尝试首位（其余模型相对顺序不变）。
// 在 chat 编排层做**本地副本**重排，不动 virtualmodel 包与策略状态——虚拟模型边界只做
// 虚拟→真实解析，粘性偏好是编排层的覆盖（策略的 UpdateRoundRobinIndex 等语义不变）。
// 未钉（pinnedModelID 为 0）或钉定模型不在映射内（映射被改/模型被删）→ 原切片返回。
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
