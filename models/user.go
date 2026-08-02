package models

import "gorm.io/gorm"

// User 管理后台登录用户（单管理员场景，username 固定 admin）。
// APIKey 用于 /v1 代理 API 鉴权（per-user，可轮换）。
type User struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	APIKey       string `gorm:"uniqueIndex"`
	Role         string `gorm:"default:'admin'"`
}
