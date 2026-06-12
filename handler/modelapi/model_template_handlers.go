package modelapi

import (
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getModelTemplateData(
	ctx *gin.Context,
	modelID uint64,
) (associations []models.ModelWithProvider, manualItems []models.ModelTemplateItem, ok bool) {
	associations, err := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", modelID).Find(ctx.Request.Context())
	if err != nil {
		common.InternalServerError(ctx, "Failed to get associations: "+err.Error())
		return nil, nil, false
	}

	manualItems, err = gorm.G[models.ModelTemplateItem](models.DB).Where("model_id = ?", modelID).Find(ctx.Request.Context())
	if err != nil {
		common.InternalServerError(ctx, "Failed to get template items: "+err.Error())
		return nil, nil, false
	}

	return associations, manualItems, true
}

func respondModelTemplate(c *gin.Context, model models.Model, modelID uint64) {
	associations, manualItems, ok := getModelTemplateData(c, modelID)
	if !ok {
		return
	}
	common.Success(c, buildModelTemplateResponse(model, associations, manualItems))
}

// GetModelTemplate 获取某个 ModelID 的模板（Model.Name + 既有关联 ProviderModel + 手动模板项）。
func GetModelTemplate(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	model, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	respondModelTemplate(c, model, id)
}

// AddModelTemplateItem 添加手动模板项（区分大小写、去重）。
func AddModelTemplateItem(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	var req ModelTemplateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.BadRequest(c, "Name is required")
		return
	}

	model, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	count, err := gorm.G[models.ModelTemplateItem](models.DB).
		Where("model_id = ? AND name = ?", id, name).
		Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	if count == 0 {
		item := models.ModelTemplateItem{
			ModelID: uint(id),
			Name:    name,
		}
		if err := gorm.G[models.ModelTemplateItem](models.DB).Create(c.Request.Context(), &item); err != nil {
			common.InternalServerError(c, "Failed to create template item: "+err.Error())
			return
		}
	}

	respondModelTemplate(c, model, id)
}

// DeleteModelTemplateItem 删除手动模板项。
func DeleteModelTemplateItem(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	var req ModelTemplateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.BadRequest(c, "Name is required")
		return
	}

	model, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	if err := models.DB.WithContext(c.Request.Context()).
		Unscoped().
		Where("model_id = ? AND name = ?", id, name).
		Delete(&models.ModelTemplateItem{}).Error; err != nil {
		common.InternalServerError(c, "Failed to delete template item: "+err.Error())
		return
	}

	respondModelTemplate(c, model, id)
}
