package pools

import "github.com/qkf688/llmux/models"

// PoolRequest 号池创建/更新请求体（设计定案第 2 节：ID / Name / Note）。
type PoolRequest struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

// PoolListItem 号池列表项：内嵌 models.Pool 保持直返实体 PascalCase 形状
// （ID/Name/Note/CreatedAt/UpdatedAt/DeletedAt），叠加号池页健康概览统计
// （设计定案第 7 节：key 数 / 健康概览 / 被引用数）。
type PoolListItem struct {
	models.Pool
	// KeyCount 该号池凭据总数（含 disabled/error 与冷却中——冷却用 CooldownUntil 表达不占状态）。
	KeyCount int64 `json:"KeyCount"`
	// StatusCounts 各状态凭据数（键为 models.CredentialStatus 常量值）。
	// 用 map 而非固定字段：S4 新增状态（如 temp_unsched）无需改响应结构。
	StatusCounts map[string]int64 `json:"StatusCounts"`
	// ReferencedBy 被分组（key_groups.PoolID）引用的次数。
	ReferencedBy int64 `json:"ReferencedBy"`
}
