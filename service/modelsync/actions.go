package modelsync

import (
	"context"

	"github.com/qkf688/llmux/models"
)

// triggerAutoActions 触发自动关联和清理操作。
func (s *Service) triggerAutoActions(ctx context.Context, hasAdded bool, hasRemoved bool) {
	if hasAdded {
		autoAssociate, _ := s.getSettingBool(ctx, models.SettingKeyAutoAssociateOnAdd)
		if autoAssociate && s.hooks.AutoAssociate != nil {
			go s.hooks.AutoAssociate(context.WithoutCancel(ctx))
		}
	}

	if hasRemoved {
		autoClean, _ := s.getSettingBool(ctx, models.SettingKeyAutoCleanOnDelete)
		if autoClean && s.hooks.CleanInvalidAssociations != nil {
			go s.hooks.CleanInvalidAssociations(context.WithoutCancel(ctx))
		}
	}
}
