package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
)

type DailyMetricsRes struct {
	Date   string `json:"date"`
	Reqs   int64  `json:"reqs"`
	Tokens int64  `json:"tokens"`
}

func MetricsDailies(c *gin.Context) {
	days, err := strconv.Atoi(c.Param("days"))
	if err != nil || days < 0 {
		httpresp.BadRequest(c, "Invalid days parameter")
		return
	}

	dailies, err := repos().Stats.ListDailiesSince(c.Request.Context(), startOfDaysAgo(time.Now(), days))
	if err != nil {
		httpresp.InternalServerError(c, "Failed to query daily metrics: "+err.Error())
		return
	}

	rows := make([]DailyMetricsRes, 0, len(dailies))
	for _, d := range dailies {
		rows = append(rows, DailyMetricsRes{Date: d.Date, Reqs: d.Reqs, Tokens: d.Tokens})
	}

	httpresp.Success(c, rows)
}
