package modelapi

import (
	"fmt"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

// GetModels 获取所有模型列表。
func GetModels(c *gin.Context) {
	modelsList, err := repos().Model.List(c.Request.Context())
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

	exists, err := repos().Model.ExistsByName(c.Request.Context(), req.Name)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if exists {
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
	if err := repos().Model.Create(c.Request.Context(), &model); err != nil {
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

	if _, ok := loadModelOrRespond(c, id); !ok {
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
	if err := repos().Model.Update(c.Request.Context(), id, &updates); err != nil {
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
	if err := deleteModelAssociations(ctx, id); err != nil {
		httpresp.InternalServerError(c, "Failed to delete model associations: "+err.Error())
		return
	}

	result, err := repos().Model.Delete(ctx, id)
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
