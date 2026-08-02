package auth

import "github.com/qkf688/llmux/repository"

// repos 返回默认 *repository.Repositories（等价 handler.Repos()，避免循环 import）。
func repos() *repository.Repositories {
	return repository.Default()
}
