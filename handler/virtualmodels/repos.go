package virtualmodels

import "github.com/atopos31/llmio/repository"

func repos() *repository.Repositories {
	return repository.Default()
}