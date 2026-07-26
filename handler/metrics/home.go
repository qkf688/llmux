package metrics

import (
	"strconv"
	"time"

	"github.com/atopos31/llmio/httpresp"
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

	sum, err := repos().Stats.SumDailiesSince(c.Request.Context(), startOfDaysAgo(time.Now(), days))
	if err != nil {
		httpresp.InternalServerError(c, "Failed to query metrics: "+err.Error())
		return
	}

	httpresp.Success(c, MetricsRes{
		Reqs:   sum.Reqs,
		Tokens: sum.Tokens,
	})
}

type Count struct {
	Model string `json:"model"`
	Calls int64  `json:"calls"`
}

func Counts(c *gin.Context) {
	rows, err := repos().Stats.ListModelCallsDesc(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	results := make([]Count, 0, len(rows))
	for _, row := range rows {
		results = append(results, Count{Model: row.Name, Calls: row.Calls})
	}

	httpresp.Success(c, results)
}

func RealModelCounts(c *gin.Context) {
	rows, err := repos().Stats.ListRealModelCallsDesc(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	results := make([]Count, 0, len(rows))
	for _, row := range rows {
		results = append(results, Count{Model: row.Name, Calls: row.Calls})
	}

	httpresp.Success(c, results)
}
