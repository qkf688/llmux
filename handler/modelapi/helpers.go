package modelapi

import (
	"context"

	"github.com/atopos31/llmio/handler/httpx"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseModelIDParam(c *gin.Context) (uint64, bool) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return 0, false
	}
	return uint64(id), true
}

func getModelByID(ctx context.Context, id uint64) (models.Model, error) {
	return gorm.G[models.Model](models.DB).Where("id = ?", id).First(ctx)
}

func deleteModelAssociations(ctx context.Context, id uint) error {
	if err := models.DB.WithContext(ctx).
		Where("model_id = ?", id).
		Delete(&models.ModelWithProvider{}).Error; err != nil {
		return err
	}
	if err := models.DB.WithContext(ctx).
		Where("model_id = ?", id).
		Delete(&models.ModelTemplateItem{}).Error; err != nil {
		return err
	}
	if err := models.DB.WithContext(ctx).
		Where("real_model_id = ?", id).
		Delete(&models.VirtualModelMapping{}).Error; err != nil {
		return err
	}
	return nil
}