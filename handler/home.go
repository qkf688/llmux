package handler

import (
	"strconv"
	"time"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type MetricsRes struct {
	Reqs   int64 `json:"reqs"`
	Tokens int64 `json:"tokens"`
}

func Metrics(c *gin.Context) {
	days, err := strconv.Atoi(c.Param("days"))
	if err != nil {
		httpresp.BadRequest(c, "Invalid days parameter")
		return
	}

	now := time.Now()
	year, month, day := now.Date()
	startDate := time.Date(year, month, day, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -days).Format("2006-01-02")

	type metricsAgg struct {
		Reqs   int64 `gorm:"column:reqs"`
		Tokens int64 `gorm:"column:tokens"`
	}
	var agg metricsAgg
	if err := models.DB.WithContext(c.Request.Context()).
		Raw("SELECT COALESCE(SUM(reqs),0) as reqs, COALESCE(SUM(tokens),0) as tokens FROM `stats_dailies` WHERE `date` >= ?", startDate).
		Scan(&agg).Error; err != nil {
		httpresp.InternalServerError(c, "Failed to query metrics: "+err.Error())
		return
	}

	httpresp.Success(c, MetricsRes{
		Reqs:   agg.Reqs,
		Tokens: agg.Tokens,
	})
}

type Count struct {
	Model string `json:"model"`
	Calls int64  `json:"calls"`
}

func Counts(c *gin.Context) {
	results := make([]Count, 0)
	if err := models.DB.Raw("SELECT name as model,calls as calls FROM `stats_model_totals` ORDER BY `calls` DESC").Scan(&results).Error; err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	httpresp.Success(c, results)
}

func RealModelCounts(c *gin.Context) {
	results := make([]Count, 0)
	if err := models.DB.Raw("SELECT name as model,calls as calls FROM `stats_real_model_totals` ORDER BY `calls` DESC").Scan(&results).Error; err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	httpresp.Success(c, results)
}
