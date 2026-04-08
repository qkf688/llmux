package handler

import (
	"errors"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func MetricsTotal(c *gin.Context) {
	var total models.StatsTotal
	err := models.DB.WithContext(c.Request.Context()).
		Model(&models.StatsTotal{}).
		Select("reqs", "tokens").
		Where("id = ?", 1).
		Take(&total).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.Success(c, MetricsRes{Reqs: 0, Tokens: 0})
			return
		}
		common.InternalServerError(c, "Failed to query total metrics: "+err.Error())
		return
	}

	common.Success(c, MetricsRes{Reqs: total.Reqs, Tokens: total.Tokens})
}

