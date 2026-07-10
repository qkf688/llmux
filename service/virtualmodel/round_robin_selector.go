package virtualmodel

import (
	"context"

	"github.com/atopos31/llmio/models"
)

func init() {
	RegisterSelector("round_robin", roundRobinSelectorInstance)
}

var roundRobinSelectorInstance = &roundRobinSelector{}

var _ Selector = (*roundRobinSelector)(nil)

type roundRobinSelector struct{}

func (r *roundRobinSelector) Select(ctx context.Context, s *Service, sc SelectionContext) (*models.Model, error) {
	return s.selectByRoundRobin(sc.VirtualModelID, sc.Mappings, sc.ModelByID)
}

func (r *roundRobinSelector) SelectOrdered(ctx context.Context, s *Service, sc SelectionContext) ([]OrderedRealModel, error) {
	return s.selectOrderedByRoundRobin(sc.VirtualModelID, sc.Mappings, sc.ModelByID)
}

func (r *roundRobinSelector) RequiresAdvanceOnSuccess() bool {
	return true
}
