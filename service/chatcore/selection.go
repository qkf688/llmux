package chatcore

import (
	"fmt"

	"github.com/qkf688/llmux/balancer"
)

// SelectByPriorityAndWeight 根据优先级和权重选择供应商。
// 优先选择优先级高的，优先级相同时按权重随机选择。
func SelectByPriorityAndWeight(weightItems map[uint]int, priorityItems map[uint]int) (*uint, error) {
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
		return balancer.WeightedRandom(highPriorityItems)
	}

	return balancer.WeightedRandom(weightItems)
}
