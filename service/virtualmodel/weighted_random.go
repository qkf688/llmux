package virtualmodel

import (
	"errors"
	"math/rand/v2"
)

// weightedRandom 权重随机选择。
func weightedRandom[T comparable](items map[T]int) (*T, error) {
	if len(items) == 0 {
		return nil, errors.New("no items to select from")
	}

	total := 0
	for _, weight := range items {
		total += weight
	}

	if total <= 0 {
		return nil, errors.New("total weight must be greater than 0")
	}

	r := rand.IntN(total)
	for key, weight := range items {
		if r < weight {
			return &key, nil
		}
		r -= weight
	}

	return nil, errors.New("unexpected error in weighted random")
}
