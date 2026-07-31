package providerapi

import "github.com/qkf688/llmux/repository"

// repos 返回默认 Repositories（经 main SetDefault / 懒创建）。
func repos() *repository.Repositories {
	return repository.Default()
}
