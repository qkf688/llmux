package models

import "gorm.io/gorm"

// ApiKey 是 /v1 代理 API 的调用 key（多 api-key 方案切片 A）：一行 = 一把 key。
// Key 存密文 hex(nonce‖ciphertext)（common/credentialcrypto，与 Credential.Key 同构，
// 支撑管理端多次复制的明文回显）；KeyHash 存 SHA-256(明文) hex，uniqueIndex 双重身份：
// /v1 鉴权按 hash 点查的入口 + 查重守卫（撞库靠 DB 唯一冲突翻译，应用层不预查——方案 §5 定案）。
// Name 允许重名（纯展示用途）；OwnerID 保留 per-user 归属语义（单管理员阶段恒指向 admin）。
// Enabled=false → /v1 鉴权 401；启停/删除不打断在途流（鉴权只在入口执行一次）。
type ApiKey struct {
	gorm.Model
	OwnerID uint   `gorm:"index;not null"`
	Name    string `gorm:"type:varchar(100);not null"`
	Key     string `gorm:"column:api_key;type:text;not null"`
	KeyHash string `gorm:"column:api_key_hash;type:varchar(64);uniqueIndex;not null"`
	// Headers 是 key 级自定义出站头（#6，方案 §2 切片 B）：网关转发 /v1 请求时按
	// 四层优先级链注入（入站透传 < key 头 < 关联级 customer_headers < providers 权威）。
	// 明文 JSON 列非密文——伪装客户端头族是展示配置不是凭据，鉴权 FindByHash 直读。
	// 与 ModelWithProvider.CustomerHeaders 同构（serializer:json；注意 map-Updates
	// 不吃 serializer，PATCH 序列化责任在 handler，见 apikeys/crud.go）。
	Headers map[string]string `gorm:"serializer:json"`
	// Enabled 不加 gorm default tag：default 标签会让「显式传 false 建行」被
	// 零值吞掉改写成 true（gorm 视为未设置），创建语义交给调用方显式赋值。
	Enabled bool `gorm:"not null"`
}

// TableName 显式固定表名，不依赖 gorm 对「ApiKey」缩写的自动蛇形转换。
func (ApiKey) TableName() string { return "api_keys" }
