package apikeys

import "github.com/qkf688/llmux/repository"

// repos 返回包级默认仓储集合（与 handler/pools、handler/auth 同一注入模式）。
func repos() *repository.Repositories {
	return repository.Default()
}
