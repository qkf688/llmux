package handler

import (
	"time"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type HourlyMetricsRes struct {
	Hour   int   `json:"hour"`
	Reqs   int64 `json:"reqs"`
	Tokens int64 `json:"tokens"`
}

func MetricsHourliesToday(c *gin.Context) {
	now := time.Now()
	year, month, day := now.Date()
	startOfToday := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	dateStr := startOfToday.Format("2006-01-02")

	type hourlyRow struct {
		Hour   int   `gorm:"column:hour"`
		Reqs   int64 `gorm:"column:reqs"`
		Tokens int64 `gorm:"column:tokens"`
	}
	rows := make([]hourlyRow, 0)
	if err := models.DB.WithContext(c.Request.Context()).
		Raw("SELECT `hour` as hour, COALESCE(reqs,0) as reqs, COALESCE(tokens,0) as tokens FROM `stats_hourlies` WHERE `date` = ? ORDER BY `hour` ASC", dateStr).
		Scan(&rows).Error; err != nil {
		httpresp.InternalServerError(c, "Failed to query hourly metrics: "+err.Error())
		return
	}

	rowMap := make(map[int]hourlyRow, len(rows))
	for _, r := range rows {
		if r.Hour < 0 || r.Hour > 23 {
			continue
		}
		rowMap[r.Hour] = r
	}

	result := make([]HourlyMetricsRes, 0, 24)
	for hour := 0; hour < 24; hour++ {
		row, ok := rowMap[hour]
		if !ok {
			result = append(result, HourlyMetricsRes{Hour: hour, Reqs: 0, Tokens: 0})
			continue
		}
		result = append(result, HourlyMetricsRes{Hour: hour, Reqs: row.Reqs, Tokens: row.Tokens})
	}

	httpresp.Success(c, result)
}
