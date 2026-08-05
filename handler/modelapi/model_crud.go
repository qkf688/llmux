package modelapi

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
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
		Name:             req.Name,
		Remark:           req.Remark,
		MaxRetry:         req.MaxRetry,
		TimeOut:          req.TimeOut,
		IOLog:            &req.IOLog,
		AutoAssociate:    req.AutoAssociate,
		SupportsThinking: req.SupportsThinking != nil && *req.SupportsThinking,
		ThinkingLevels:   req.ThinkingLevels,
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

	updates := map[string]any{
		"name":      req.Name,
		"remark":    req.Remark,
		"max_retry": req.MaxRetry,
		"time_out":  req.TimeOut,
		"io_log":    req.IOLog,
	}
	if req.AutoAssociate != nil {
		updates["auto_associate"] = *req.AutoAssociate
	}
	// 条件写：缺省（nil）时保留存量值，避免 PUT 不带该字段把已开启的模型静默重置为 false
	if req.SupportsThinking != nil {
		updates["supports_thinking"] = *req.SupportsThinking
	}
	// ThinkingLevels：DTO 用 []string（非指针），nil=不约束（空切片语义）。
	// 始终写入：前端不发字段时 JSON 解析为 nil，写入空切片（=不约束）；
	// 前端发非空数组时写入白名单。与 SupportsThinking 的条件写不同——
	// ThinkingLevels 无"不改"语义（前端总是显式提交完整列表）。
	// 需手动 JSON 序列化：GORM serializer:json 只对 struct Updates 生效，
	// map-based UpdateFields 不走 serializer，直接传 []string 会报 unsupported type。
	thinkingLevelsVal, err := models.SerializeThinkingLevelsForUpdate(req.ThinkingLevels)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to serialize thinking_levels: "+err.Error())
		return
	}
	updates["thinking_levels"] = thinkingLevelsVal
	if _, err := repos().Model.UpdateFields(c.Request.Context(), id, updates); err != nil {
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
	r := repos()
	var result int64
	err := r.RunInTx(ctx, func(tx *repository.Repositories) error {
		if err := deleteModelAssociations(ctx, tx, id); err != nil {
			return err
		}
		affected, err := tx.Model.Delete(ctx, id)
		if err != nil {
			return err
		}
		result = affected
		return nil
	})
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
