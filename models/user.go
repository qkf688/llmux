package models

import "gorm.io/gorm"

// User 管理后台登录用户（单管理员场景，username 固定 admin）。
// /v1 调用凭据不在本实体：多 api-key 方案起，调用 key 是 api_keys 表的独立行。
type User struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"default:'admin'"`
}
