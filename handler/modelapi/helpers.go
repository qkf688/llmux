package modelapi

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

func parseModelIDParam(c *gin.Context) (uint, bool) {
	return httpx.ParseUintParamAllowZero(c, "id")
}

func getModelByID(ctx context.Context, id uint) (models.Model, error) {
	model, err := repos().Model.Get(ctx, id)
	if err != nil {
		return models.Model{}, err
	}
	return *model, nil
}

// deleteModelAssociations 级联清理模型的下游引用：关联、模板项、虚拟模型映射。
// 接受 *Repositories 参数以便在调用方的事务内执行（参见 Repositories.RunInTx）。
func deleteModelAssociations(ctx context.Context, r *repository.Repositories, id uint) error {
	if _, err := r.ModelWithProvider.DeleteByModelID(ctx, id); err != nil {
		return err
	}
	if _, err := r.ModelTemplateItem.DeleteByModelIDUnscoped(ctx, id); err != nil {
		return err
	}
	if _, err := r.VirtualModelMapping.DeleteByRealModelID(ctx, id); err != nil {
		return err
	}
	return nil
}
