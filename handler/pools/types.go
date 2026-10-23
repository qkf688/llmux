package pools

import (
	"github.com/qkf688/llmux/models"
)

// PoolRequest 号池创建/更新请求体（name / note / test_model）。
// test_model 为池级「测试模型」（#24-B 迁移）：按凭据测试打真实 completion 用的模型名。
type PoolRequest struct {
	Name      string `json:"name"`
	Note      string `json:"note"`
	TestModel string `json:"test_model"`
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
	// RefGroups 引用该号池的分组明细（供应商名 + 分组名），号池页引用区
	// chips 与双向导航数据源。未被引用时为空数组（非 null）。
	RefGroups []PoolRefGroup `json:"RefGroups"`
}

// PoolRefGroup 号池被引用明细的一条：哪个供应商的哪个分组引用了该号池。
// 供应商已被删除但分组残留（DeleteProvider 不级联删分组）时 ProviderName
// 为空串，前端按需降级展示。
type PoolRefGroup struct {
	ProviderID   uint   `json:"ProviderID"`
	ProviderName string `json:"ProviderName"`
	GroupID      uint   `json:"GroupID"`
	GroupName    string `json:"GroupName"`
}

// CredentialTestResult 号池详情「按凭据测试」（#24-B）响应：与 handler/testapi 四个连通测试
// 端点同族形状——一次失败的测试仍是 code 200 + passed:false，结论只可能在 data 里
// （前端 apiRequest 在 code!=200 时只抛 message、丢 data，用错误信封回传等于前端读不到）。
//
// 只报不改：本端点绝不写回 Credential.Status/FailCount/Cooldown，纯探活结论。
// group_id/group_name 为实际命中的引用分组（取 ProviderID/GroupID 最小者）；
// 链路未选定（池无引用组）时为 0/空串，error_type=provider。model 为本次点测实际使用的
// 测试模型（来自号池 test_model；未配置/链路未选定时为空串）。
type CredentialTestResult struct {
	Passed     bool   `json:"passed"`
	GroupID    uint   `json:"group_id"`
	GroupName  string `json:"group_name"`
	ProviderID uint   `json:"provider_id"`
	Model      string `json:"model,omitempty"`
	Error      string `json:"error,omitempty"`
	ErrorType  string `json:"error_type,omitempty"`
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

// AutoPoolImportRequest 「自动池 + 凭据」一步式落库的请求体。
// Name 由前端按「供应商名·分组名」拼好（命名口径单一来源在前端），后端只负责重名时
// 加序号后缀兜底，保证「同名不报错、池名唯一」。
type AutoPoolImportRequest struct {
	Name string   `json:"name"`
	Note string   `json:"note"`
	Keys []string `json:"keys"`
}

// AutoPoolImportData 自动池创建 + 凭据导入结果：池标识 + 批量导入逐行回显。
// 内嵌 BatchImportData（汇总计数与逐行回显的契约与既有批量导入完全同源）。
type AutoPoolImportData struct {
	PoolID   uint   `json:"pool_id"`
	PoolName string `json:"pool_name"`
	BatchImportData
}
