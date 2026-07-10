// Package repository 封装对持久层（GORM/SQLite）的访问，为 service/handler 提供与数据访问框架解耦的接口。
package repository

import "gorm.io/gorm"

// Repositories 聚合所有 Repository 实例，便于在应用启动时统一初始化与注入。
type Repositories struct {
	Provider              ProviderRepo
	Model                 ModelRepo
	VirtualModel          VirtualModelRepo
	VirtualModelMapping   VirtualModelMappingRepo
	Setting               SettingRepo
}

// New 根据 *gorm.DB 创建所有 Repository 实例。
func New(db *gorm.DB) *Repositories {
	return &Repositories{
		Provider:            NewProviderRepo(db),
		Model:               NewModelRepo(db),
		VirtualModel:        NewVirtualModelRepo(db),
		VirtualModelMapping: NewVirtualModelMappingRepo(db),
		Setting:             NewSettingRepo(db),
	}
}
