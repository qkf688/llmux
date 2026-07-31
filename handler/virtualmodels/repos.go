package virtualmodels

import "github.com/qkf688/llmux/repository"

func repos() *repository.Repositories {
	return repository.Default()
}
