package models

import (
	"time"

	"gorm.io/gorm"
)

// Pool 号池：一批凭据的复用容器（纯凭据集合，不含 URL/协议）。
// 设计定案第 2 节：pools = ID / Name / Note。
type Pool struct {
	gorm.Model
	Name string `gorm:"type:varchar(255);uniqueIndex;not null"`
	Note string `gorm:"type:text"`
}

// CredentialStatus 凭据状态枚举。冷却不占状态（用 CooldownUntil 表达），
// 与设计定案第 5 节状态机一致：active → 冷却(cooldown_until) → 到期自动回 active。
const (
	CredentialStatusActive   = "active"
	CredentialStatusDisabled = "disabled"
	CredentialStatusError    = "error"
)

// Credential 凭据：一条 = 一个 key，归属号池（PoolID）或分组内联（GroupID）二选一。
// Key 存密文（hex(nonce‖ciphertext)，见 common/credentialcrypto），
// KeyHash 用于不解密即可完成的批量导入去重 / 搜索 / 日志关联。
// GroupID / PoolID 为三态指针：nil 表示未归属该侧（设计语义「二选一」由调用层保证）。
type Credential struct {
	gorm.Model
	Key     string `gorm:"type:text;not null"`
	KeyHash string `gorm:"type:varchar(64);index"`
	Note    string `gorm:"type:text"`
	GroupID *uint  `gorm:"index"`
	PoolID  *uint  `gorm:"index"`
	Status  string `gorm:"type:varchar(20);default:'active';index"`
	// 冷却状态机字段（设计定案第 5 节）：CooldownUntil 到期自动恢复 active；
	// 三级冷却共用一个窗口字段 + reason（无 OAuth/订阅场景，429 与过载语义合并）。
	CooldownUntil  *time.Time
	CooldownReason string `gorm:"type:text"`
	FailCount      int
	LastUsedAt     *time.Time
	// 统计字段（号池页概览用；S3/S4 起写入）
	TotalRequests int64
	TotalErrors   int64
	TotalTokens   int64
}

// Endpoint 协议端点：每个供应商每个协议一条（设计定案「不做同协议多 URL」）。
// URL 留空 = 继承 Provider 的 base_url（字段级继承链第 3 节）；填 = 覆盖。
type Endpoint struct {
	gorm.Model
	ProviderID uint   `gorm:"index;not null"`
	Protocol   string `gorm:"type:varchar(20);not null"`
	URL        string `gorm:"type:text"`
	Enabled    bool   `gorm:"default:true"`
}

// KeyGroup 凭据分组：白名单过滤 + 价格权重选择（设计定案第 4 节）。
// 凭据来源二选一：关联号池（PoolID）或分组内联凭据（credentials.GroupID）。
type KeyGroup struct {
	gorm.Model
	ProviderID uint   `gorm:"index;not null"`
	Name       string `gorm:"type:varchar(255)"`
	Weight     int    `gorm:"default:1"` // 价格导向权重：便宜的分组权重高
	Models     string `gorm:"type:text"` // 模型白名单，逗号分隔，空 = 不限
	PoolID     *uint  `gorm:"index"`
}
