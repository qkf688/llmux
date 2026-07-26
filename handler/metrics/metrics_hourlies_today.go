package metrics

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
	rows, err := repos().Stats.ListHourliesByDate(c.Request.Context(), startOfDay(time.Now()))
	if err != nil {
		httpresp.InternalServerError(c, "Failed to query hourly metrics: "+err.Error())
		return
	}

	httpresp.Success(c, fillFullDay(rows))
}

// fillFullDay 把稀疏的小时统计补齐为 0-23 共 24 项，缺失小时以零值填充。
// 纯函数：前端图表要求整日刻度完整，该补零属展示逻辑，不下沉仓储。
func fillFullDay(rows []models.StatsHourly) []HourlyMetricsRes {
	rowMap := make(map[int]models.StatsHourly, len(rows))
	for _, r := range rows {
		if r.Hour < 0 || r.Hour > 23 {
			continue
		}
		rowMap[r.Hour] = r
	}

	result := make([]HourlyMetricsRes, 0, 24)
	for hour := range 24 {
		row := rowMap[hour] // 缺失时为零值，Reqs/Tokens 即 0
		result = append(result, HourlyMetricsRes{Hour: hour, Reqs: row.Reqs, Tokens: row.Tokens})
	}
	return result
}
