package chatcore

import (
	"fmt"

	"github.com/qkf688/llmux/balancer"
)

// SelectByPriorityAndWeight 根据优先级和权重选择供应商。
// 优先选择优先级高的，优先级相同时按权重随机选择。
//
// preferID 是缓存亲和的钉定偏好（0 = 不钉）：仅当它在「最高优先级档内（档 map 非空时）
// 或无档时的全池内」且权重 > 0 时直返——优先级是管理员的策略意志，粘性不越权（本条
// 只约束**候选层**；虚拟模型层的真实模型前移刻意不做档约束，见 chat_affinity.go 的
// preferRealModelsFirst 注释 Ruling）；权重 0 的既有语义是「不参与」，粘性也不得复活它。
// 其余情形（掉出档外/池外/权重 0）不钉、按现状选择。钉定直返不推进任何状态（本函数
// 无状态，RR 在 channel 层同理不推进）。
func SelectByPriorityAndWeight(weightItems map[uint]int, priorityItems map[uint]int, preferID uint) (*uint, error) {
	if len(weightItems) == 0 {
		return nil, fmt.Errorf("no provide items")
	}

	maxPriority := -1
	for id := range weightItems {
		if priority, ok := priorityItems[id]; ok && priority > maxPriority {
			maxPriority = priority
		}
	}

	highPriorityItems := make(map[uint]int)
	for id, weight := range weightItems {
		if priority, ok := priorityItems[id]; ok && priority == maxPriority {
			highPriorityItems[id] = weight
		}
	}

	if len(highPriorityItems) > 0 {
		if preferID != 0 {
			if w, ok := highPriorityItems[preferID]; ok && w > 0 {
				return &preferID, nil
			}
		}
		return balancer.WeightedRandom(highPriorityItems)
	}

	if preferID != 0 {
		if w, ok := weightItems[preferID]; ok && w > 0 {
			return &preferID, nil
		}
	}
	return balancer.WeightedRandom(weightItems)
}
