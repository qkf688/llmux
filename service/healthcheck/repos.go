package healthcheck

import "github.com/atopos31/llmio/repository"

// repos 返回默认 *repository.Repositories。
// 健康检测的持久层访问统一经此入口，禁止再直连 models.DB。
func repos() *repository.Repositories {
	return repository.Default()
}
