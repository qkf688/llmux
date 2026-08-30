package pools

import (
	"time"

	"github.com/qkf688/llmux/models"
)

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

// CredentialListItem 凭据列表项：掩码后的展示形态，不含明文/密文 Key。
type CredentialListItem struct {
	ID             uint       `json:"ID"`
	PoolID         *uint      `json:"PoolID"`
	GroupID        *uint      `json:"GroupID"`
	Status         string     `json:"Status"`
	Note           string     `json:"Note"`
	KeyMasked      string     `json:"KeyMasked"`
	CooldownUntil  *time.Time `json:"CooldownUntil"`
	CooldownReason string     `json:"CooldownReason"`
	FailCount      int        `json:"FailCount"`
	LastUsedAt     *time.Time `json:"LastUsedAt"`
	TotalRequests  int64      `json:"TotalRequests"`
	TotalErrors    int64      `json:"TotalErrors"`
	TotalTokens    int64      `json:"TotalTokens"`
	CreatedAt      time.Time  `json:"CreatedAt"`
	UpdatedAt      time.Time  `json:"UpdatedAt"`
}

// CredentialListData 凭据列表分页信封。
type CredentialListData struct {
	Items    []CredentialListItem `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// CreateCredentialRequest 凭据创建请求体（设计定案：每条 = 一个明文 key）。
type CreateCredentialRequest struct {
	Key  string `json:"key"`
	Note string `json:"note"`
}

// UpdateCredentialRequest 凭据单条更新请求体（note 可清空，status 可切）。
// 均用指针区分「字段缺席」与「显式置空/改值」。
type UpdateCredentialRequest struct {
	Note   *string `json:"note"`
	Status *string `json:"status"`
}

// BatchCredentialStatusRequest 批量启停请求体。
type BatchCredentialStatusRequest struct {
	IDs    []uint `json:"ids"`
	Status string `json:"status"`
}

// BatchCredentialDeleteRequest 批量删除请求体。
type BatchCredentialDeleteRequest struct {
	IDs []uint `json:"ids"`
}

// CredentialRawData 明文查看响应（受控端点 GET .../raw）。
type CredentialRawData struct {
	Key string `json:"key"`
}

// BatchImportCredentialsRequest 批量粘贴导入请求体（设计定案第 7 节：
// 前端按行拆分后传数组，服务端不做文本解析）。
type BatchImportCredentialsRequest struct {
	Keys []string `json:"keys"`
}

// BatchImportRow 批量导入逐行回显：与输入 keys 按 Index 一一对应。
// Key 为掩码形态（空行回空串）；Status ∈ imported|skipped|failed（机器码，
// 前端映射展示文案）；Reason 仅 skipped/failed 行存在（duplicate_in_batch 等机器码）。
type BatchImportRow struct {
	Index  int    `json:"Index"`
	Key    string `json:"Key"`
	Status string `json:"Status"`
	Reason string `json:"Reason,omitempty"`
}

// BatchImportData 批量导入响应：展示用汇总计数 + 逐行回显（失败行回显）。
type BatchImportData struct {
	Total    int              `json:"total"`
	Imported int              `json:"imported"`
	Skipped  int              `json:"skipped"`
	Failed   int              `json:"failed"`
	Rows     []BatchImportRow `json:"rows"`
}
