package modelapi

import (
	"fmt"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetModels 获取所有模型列表。
func GetModels(c *gin.Context) {
	modelsList, err := gorm.G[models.Model](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, modelsList)
}

// CreateModel 创建模型。
func CreateModel(c *gin.Context) {
	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	count, err := gorm.G[models.Model](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count > 0 {
		common.BadRequest(c, fmt.Sprintf("Model: %s already exists", req.Name))
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
		common.InternalServerError(c, "Failed to create model: "+err.Error())
		return
	}

	common.Success(c, model)
}

// UpdateModel 更新模型。
func UpdateModel(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	_, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
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
		common.InternalServerError(c, "Failed to update model: "+err.Error())
		return
	}

	updatedModel, err := getModelByID(c.Request.Context(), id)
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve updated model: "+err.Error())
		return
	}

	common.Success(c, updatedModel)
}

// DeleteModel 删除模型。
func DeleteModel(c *gin.Context) {
	id, ok := parseModelIDParam(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := deleteModelAssociations(ctx, uint(id)); err != nil {
		common.InternalServerError(c, "Failed to delete model associations: "+err.Error())
		return
	}

	result, err := gorm.G[models.Model](models.DB).Where("id = ?", id).Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to delete model: "+err.Error())
		return
	}
	if result == 0 {
		common.NotFound(c, "Model not found")
		return
	}

	common.Success(c, nil)
}
