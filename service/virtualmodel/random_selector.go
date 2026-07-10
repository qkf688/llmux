package virtualmodel

import (
	"context"

	"github.com/atopos31/llmio/models"
)

func init() {
	RegisterSelector("random", randomSelectorInstance)
}

var randomSelectorInstance = &randomSelector{}

var _ Selector = (*randomSelector)(nil)

type randomSelector struct{}

func (r *randomSelector) Select(ctx context.Context, s *Service, sc SelectionContext) (*models.Model, error) {
	return s.selectByRandom(sc.Mappings, sc.ModelByID)
}

func (r *randomSelector) SelectOrdered(ctx context.Context, s *Service, sc SelectionContext) ([]OrderedRealModel, error) {
	return s.selectOrderedByRandom(sc.Mappings, sc.ModelByID)
}

func (r *randomSelector) RequiresAdvanceOnSuccess() bool {
	return false
}
