package handler

import (
	"context"

	"github.com/atopos31/llmio/handler/autoassoc"
)

// AssociationPreview 关联预览信息（兼容层）。
type AssociationPreview = autoassoc.AssociationPreview

// PreviewAutoAssociate 预览将要添加的关联（兼容层）。
var PreviewAutoAssociate = autoassoc.PreviewAutoAssociate

// PreviewCleanInvalid 预览将要删除的关联（兼容层）。
var PreviewCleanInvalid = autoassoc.PreviewCleanInvalid

// AutoAssociateModels 一键添加关联（兼容层）。
var AutoAssociateModels = autoassoc.AutoAssociateModels

// CleanInvalidAssociations 清除无效关联（兼容层）。
var CleanInvalidAssociations = autoassoc.CleanInvalidAssociations

// triggerAutoAssociate 触发自动关联（兼容层）。
func triggerAutoAssociate(ctx context.Context) {
	autoassoc.TriggerAutoAssociate(ctx)
}

// triggerAutoClean 触发自动清理（兼容层）。
func triggerAutoClean(ctx context.Context) {
	autoassoc.TriggerAutoClean(ctx)
}
