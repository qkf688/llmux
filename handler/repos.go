package handler

import "github.com/qkf688/llmux/repository"

// Repos 返回默认 *repository.Repositories。
// 子包拆分后生产路径优先走此入口（或子包内等价的 repository.Default()），
// 避免再引入第二套全局 models.DB CRUD 习惯。
//
// 初始化约定：
//   - 正常启动：models.Init 之后 repository.Default() 懒创建；
//   - 测试或显式注入：repository.SetDefault(repository.New(db))。
func Repos() *repository.Repositories {
	return repository.Default()
}
