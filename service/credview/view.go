// View 凭据对外视图（管理端 JSON 形状）的唯一来源。
//
// 为什么单独成包：凭据视图原先是两处各拼一份——号池域 `pools.CredentialListItem` 与
// 供应商分组的掩码项。同一把凭据两个形状，加字段（如状态、冷却）必定漏一处。
// 凭据提为第一等资源后（`handler/credentials`），两个域都从这里取同一个视图：
// 新增凭据级字段只改本文件。
package credview

import (
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
)

// View 凭据视图：掩码后的展示形态，不含明文/密文/KeyHash（明文只经受控 raw 端点显式拉取）。
// 字段以 PascalCase 对管理端前端暴露。三态指针（CooldownUntil/...）**禁 omitempty**：
// nil 是「无该归属 / 无冷却」的有效语义，序列化成 JSON null，前端据此区分状态。
type View struct {
	ID             uint       `json:"ID"`
	PoolID         uint       `json:"PoolID"`
	Status         string     `json:"Status"`
	Note           string     `json:"Note"`
	KeyMasked      string     `json:"KeyMasked"`
	CooldownUntil  *time.Time `json:"CooldownUntil"`
	CooldownReason string     `json:"CooldownReason"`
	FailCount      int        `json:"FailCount"`
	LastUsedAt     *time.Time `json:"LastUsedAt"`
	LastProbeAt    *time.Time `json:"LastProbeAt"`
	TotalRequests  int64      `json:"TotalRequests"`
	TotalErrors    int64      `json:"TotalErrors"`
	TotalTokens    int64      `json:"TotalTokens"`
	CreatedAt      time.Time  `json:"CreatedAt"`
	UpdatedAt      time.Time  `json:"UpdatedAt"`
}

// ViewOf 把凭据实体映射为对外视图。掩码与降级策略（cipher 未配置 / 解密失败 → `****` +
// slog.Warn）走 credentialcrypto.MaskStored 单源——两个消费域共用同一降级行为，
// 改规则只动 MaskStored 一处，且绝不输出密文片段。
func ViewOf(cred models.Credential) View {
	return View{
		ID:             cred.ID,
		PoolID:         cred.PoolID,
		Status:         cred.Status,
		Note:           cred.Note,
		KeyMasked:      credentialcrypto.MaskStored(credentialcrypto.Default(), cred.Key, "credential_id", cred.ID),
		CooldownUntil:  cred.CooldownUntil,
		CooldownReason: cred.CooldownReason,
		FailCount:      cred.FailCount,
		LastUsedAt:     cred.LastUsedAt,
		LastProbeAt:    cred.LastProbeAt,
		TotalRequests:  cred.TotalRequests,
		TotalErrors:    cred.TotalErrors,
		TotalTokens:    cred.TotalTokens,
		CreatedAt:      cred.CreatedAt,
		UpdatedAt:      cred.UpdatedAt,
	}
}

// ViewsOf 批量映射。空输入回空切片（JSON 出 `[]` 而非 null，前端列表无需判空分支）。
func ViewsOf(creds []models.Credential) []View {
	views := make([]View, 0, len(creds))
	for _, cred := range creds {
		views = append(views, ViewOf(cred))
	}
	return views
}
