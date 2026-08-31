package credprobe

import "github.com/qkf688/llmux/repository"

// repos 返回默认 *repository.Repositories。
// 探活写库统一经此入口，禁止再直连 models.DB。
func repos() *repository.Repositories {
	return repository.Default()
}
