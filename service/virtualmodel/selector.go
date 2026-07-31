package virtualmodel

import (
	"context"

	"github.com/qkf688/llmux/models"
)

// SelectionContext 传递给 Selector 的上下文信息。
type SelectionContext struct {
	VirtualModelID uint
	Mappings       []models.VirtualModelMapping
	ModelByID      map[uint]models.Model
}

// Selector 定义虚拟模型选择策略。
// 每个策略只需实现该接口并在 init() 中注册即可被服务使用。
type Selector interface {
	// Select 从候选池中选择一个真实模型。
	Select(ctx context.Context, s *Service, sc SelectionContext) (*models.Model, error)
	// SelectOrdered 返回有序的真实模型列表，用于故障转移。
	SelectOrdered(ctx context.Context, s *Service, sc SelectionContext) ([]OrderedRealModel, error)
	// RequiresAdvanceOnSuccess 返回请求成功后是否需要推进内部状态（如 round_robin）。
	RequiresAdvanceOnSuccess() bool
}

var selectors = make(map[string]Selector)

// RegisterSelector 注册一个虚拟模型选择策略。
// 重复注册同一策略会 panic，避免运行期覆盖导致行为不可预期。
func RegisterSelector(strategy string, s Selector) {
	if _, exists := selectors[strategy]; exists {
		panic("selector already registered: " + strategy)
	}
	selectors[strategy] = s
}

// GetSelector 获取已注册的选择策略；若未找到则返回默认 priority 策略与 false。
func GetSelector(strategy string) (Selector, bool) {
	s, ok := selectors[strategy]
	if !ok {
		s, _ = selectors["priority"]
		return s, false
	}
	return s, true
}
