package chatstats

import "github.com/qkf688/llmux/repository"

// repos 返回默认 *repository.Repositories。
// 统计写入统一经此入口访问持久层，禁止再直连 models.DB。
func repos() *repository.Repositories {
	return repository.Default()
}
