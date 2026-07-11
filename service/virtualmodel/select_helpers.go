package virtualmodel

import "github.com/atopos31/llmio/models"

// buildOrderedRealModel 从 mapping 和 model 构建 OrderedRealModel。
// select_ordered.go 中 3 个策略函数均使用此函数，消除重复构建逻辑。
func buildOrderedRealModel(mapping models.VirtualModelMapping, model models.Model) OrderedRealModel {
	return OrderedRealModel{
		Model:    model,
		Priority: mapping.Priority,
		Weight:   mapping.Weight,
	}
}

// findModelByID 从 modelByID 映射中查找模型。未找到时返回 false。
// select_single.go 中多处使用此查找模式，统一为辅助函数。
func findModelByID(id uint, modelByID map[uint]models.Model) (models.Model, bool) {
	model, ok := modelByID[id]
	return model, ok
}
