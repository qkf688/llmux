package virtualmodel

import "github.com/qkf688/llmux/models"

// OrderedRealModel 有序的真实模型（用于虚拟模型故障转移）。
type OrderedRealModel struct {
	Model    models.Model
	Priority int
	Weight   int
}

type candidatePool struct {
	mappings  []models.VirtualModelMapping
	modelByID map[uint]models.Model
}
