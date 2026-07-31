package virtualmodel

import (
	"context"
	"fmt"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// ValidateNoCircularDependency 验证没有循环依赖。
func (s *Service) ValidateNoCircularDependency(ctx context.Context, virtualModelID uint, realModelID uint) error {
	realModel, err := gorm.G[models.Model](s.db).Where("id = ?", realModelID).First(ctx)
	if err != nil {
		return fmt.Errorf("failed to get real model: %w", err)
	}

	count, err := gorm.G[models.VirtualModel](s.db).
		Where("name = ? AND id != ?", realModel.Name, virtualModelID).
		Count(ctx, "id")
	if err != nil {
		return fmt.Errorf("failed to check virtual model: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("cannot associate model '%s': a virtual model with the same name exists", realModel.Name)
	}

	return nil
}
