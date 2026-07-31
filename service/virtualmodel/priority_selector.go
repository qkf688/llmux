package virtualmodel

import (
	"context"

	"github.com/qkf688/llmux/models"
)

func init() {
	RegisterSelector("priority", prioritySelectorInstance)
}

var prioritySelectorInstance = &prioritySelector{}

var _ Selector = (*prioritySelector)(nil)

type prioritySelector struct{}

func (p *prioritySelector) Select(ctx context.Context, s *Service, sc SelectionContext) (*models.Model, error) {
	return s.selectByPriority(sc.Mappings, sc.ModelByID)
}

func (p *prioritySelector) SelectOrdered(ctx context.Context, s *Service, sc SelectionContext) ([]OrderedRealModel, error) {
	return s.selectOrderedByPriority(sc.Mappings, sc.ModelByID)
}

func (p *prioritySelector) RequiresAdvanceOnSuccess() bool {
	return false
}
