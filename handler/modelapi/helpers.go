package modelapi

import (
	"context"

	"github.com/atopos31/llmio/handler/httpx"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
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
func deleteModelAssociations(ctx context.Context, id uint) error {
	r := repos()
	if _, err := r.ModelWithProvider.DeleteByModelID(ctx, id); err != nil {
		return err
	}
	if _, err := r.ModelTemplateItem.DeleteByModelID(ctx, id); err != nil {
		return err
	}
	if _, err := r.VirtualModelMapping.DeleteByRealModelID(ctx, id); err != nil {
		return err
	}
	return nil
}
