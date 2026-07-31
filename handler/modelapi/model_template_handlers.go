package modelapi

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func getModelTemplateData(
	ctx *gin.Context,
	modelID uint,
) (associations []models.ModelWithProvider, manualItems []models.ModelTemplateItem, ok bool) {
	associations, err := repos().ModelWithProvider.ListByModelID(ctx.Request.Context(), modelID)
	if err != nil {
		httpresp.InternalServerError(ctx, "Failed to get associations: "+err.Error())
		return nil, nil, false
	}

	manualItems, err = repos().ModelTemplateItem.ListByModelID(ctx.Request.Context(), modelID)
	if err != nil {
		httpresp.InternalServerError(ctx, "Failed to get template items: "+err.Error())
		return nil, nil, false
	}

	return associations, manualItems, true
}

func respondModelTemplate(c *gin.Context, model models.Model, modelID uint) {
	associations, manualItems, ok := getModelTemplateData(c, modelID)
	if !ok {
		return
	}
	httpresp.Success(c, buildModelTemplateResponse(model, associations, manualItems))
}

// loadModelOrRespond 取回模型，未找到/出错时已写好响应并返回 false。
func loadModelOrRespond(c *gin.Context, id uint) (models.Model, bool) {
	model, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Model not found")
			return models.Model{}, false
		}
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return models.Model{}, false
	}
	return model, true
}

// bindTemplateItemName 绑定并校验模板项名称。
func bindTemplateItemName(c *gin.Context) (string, bool) {
	var req ModelTemplateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpresp.BadRequest(c, "Name is required")
		return "", false
	}
	return name, true
}

// GetModelTemplate 获取某个 ModelID 的模板（Model.Name + 既有关联 ProviderModel + 手动模板项）。
func GetModelTemplate(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	model, ok := loadModelOrRespond(c, id)
	if !ok {
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

	name, ok := bindTemplateItemName(c)
	if !ok {
		return
	}

	model, ok := loadModelOrRespond(c, id)
	if !ok {
		return
	}

	count, err := repos().ModelTemplateItem.CountByModelIDAndName(c.Request.Context(), id, name)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	if count == 0 {
		item := models.ModelTemplateItem{
			ModelID: id,
			Name:    name,
		}
		if err := repos().ModelTemplateItem.Create(c.Request.Context(), &item); err != nil {
			httpresp.InternalServerError(c, "Failed to create template item: "+err.Error())
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

	name, ok := bindTemplateItemName(c)
	if !ok {
		return
	}

	model, ok := loadModelOrRespond(c, id)
	if !ok {
		return
	}

	if _, err := repos().ModelTemplateItem.DeleteByModelIDAndNameUnscoped(c.Request.Context(), id, name); err != nil {
		httpresp.InternalServerError(c, "Failed to delete template item: "+err.Error())
		return
	}

	respondModelTemplate(c, model, id)
}
