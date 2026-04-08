package handler

import (
	"strconv"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type DailyMetricsRes struct {
	Date   string `json:"date"`
	Reqs   int64  `json:"reqs"`
	Tokens int64  `json:"tokens"`
}

func MetricsDailies(c *gin.Context) {
	days, err := strconv.Atoi(c.Param("days"))
	if err != nil || days < 0 {
		common.BadRequest(c, "Invalid days parameter")
		return
	}

	now := time.Now()
	year, month, day := now.Date()
	startDate := time.Date(year, month, day, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -days).Format("2006-01-02")

	rows := make([]DailyMetricsRes, 0)
	if err := models.DB.WithContext(c.Request.Context()).
		Raw("SELECT `date` as date, COALESCE(reqs,0) as reqs, COALESCE(tokens,0) as tokens FROM `stats_dailies` WHERE `date` >= ? ORDER BY `date` ASC", startDate).
		Scan(&rows).Error; err != nil {
		common.InternalServerError(c, "Failed to query daily metrics: "+err.Error())
		return
	}

	common.Success(c, rows)
}

