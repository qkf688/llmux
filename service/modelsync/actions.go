package modelsync

import (
	"context"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
)

// triggerAutoActions 触发自动关联和清理操作。
// 经 bgtask 登记：其 ctx 本就不随请求/同步取消（等价于原 context.WithoutCancel），
// 且进程关闭时会等这两个写库动作跑完。
func (s *Service) triggerAutoActions(ctx context.Context, hasAdded bool, hasRemoved bool) {
	if hasAdded {
		autoAssociate, _ := s.getSettingBool(ctx, models.SettingKeyAutoAssociateOnAdd)
		if autoAssociate && s.hooks.AutoAssociate != nil {
			bgtask.Go(s.hooks.AutoAssociate)
		}
	}

	if hasRemoved {
		autoClean, _ := s.getSettingBool(ctx, models.SettingKeyAutoCleanOnDelete)
		if autoClean && s.hooks.CleanInvalidAssociations != nil {
			bgtask.Go(s.hooks.CleanInvalidAssociations)
		}
	}
}
