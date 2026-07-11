package providerapi

import "github.com/atopos31/llmio/repository"

// repos 返回默认 Repositories（经 main SetDefault / 懒创建）。
func repos() *repository.Repositories {
	return repository.Default()
}