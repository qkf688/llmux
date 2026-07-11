package modelapi

import (
	"fmt"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetModels 获取所有模型列表。
func GetModels(c *gin.Context) {
	modelsList, err := gorm.G[models.Model](models.DB).Find(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	httpresp.Success(c, modelsList)
}

// CreateModel 创建模型。
func CreateModel(c *gin.Context) {
	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	count, err := gorm.G[models.Model](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count > 0 {
		httpresp.BadRequest(c, fmt.Sprintf("Model: %s already exists", req.Name))
		return
	}

	model := models.Model{
		Name:          req.Name,
		Remark:        req.Remark,
		MaxRetry:      req.MaxRetry,
		TimeOut:       req.TimeOut,
		IOLog:         &req.IOLog,
		AutoAssociate: req.AutoAssociate,
	}
	if err := gorm.G[models.Model](models.DB).Create(c.Request.Context(), &model); err != nil {
		httpresp.InternalServerError(c, "Failed to create model: "+err.Error())
		return
	}

	httpresp.Success(c, model)
}

// UpdateModel 更新模型。
func UpdateModel(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	_, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httpresp.NotFound(c, "Model not found")
			return
		}
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	updates := models.Model{
		Name:          req.Name,
		Remark:        req.Remark,
		MaxRetry:      req.MaxRetry,
		TimeOut:       req.TimeOut,
		IOLog:         &req.IOLog,
		AutoAssociate: req.AutoAssociate,
	}
	if _, err := gorm.G[models.Model](models.DB).Where("id = ?", id).Updates(c.Request.Context(), updates); err != nil {
		httpresp.InternalServerError(c, "Failed to update model: "+err.Error())
		return
	}

	updatedModel, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve updated model: "+err.Error())
		return
	}

	httpresp.Success(c, updatedModel)
}

// DeleteModel 删除模型。
func DeleteModel(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := deleteModelAssociations(ctx, uint(id)); err != nil {
		httpresp.InternalServerError(c, "Failed to delete model associations: "+err.Error())
		return
	}

	result, err := gorm.G[models.Model](models.DB).Where("id = ?", id).Delete(ctx)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete model: "+err.Error())
		return
	}
	if result == 0 {
		httpresp.NotFound(c, "Model not found")
		return
	}

	httpresp.Success(c, nil)
}
