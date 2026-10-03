package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
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
		httpresp.InternalErrorMessage(c, "Failed to query metrics", err)
		return
	}

	httpresp.Success(c, MetricsRes{
		Reqs:   sum.Reqs,
		Tokens: sum.Tokens,
	})
}

// Count 仪表盘真实命中模型排行的一行。
//
// 三个 token 字段恒为非负整数、无三态语义，故不设指针也不用 omitempty：
// 「该模型没有 token 记录」（早于 token 统计起点的历史调用）与「token 确为 0」
// 在库里无法区分，读侧一律按「未知」呈现，故协议层原样透出 0 由前端决定怎么显示。
type Count struct {
	Model            string `json:"model"`
	Calls            int64  `json:"calls"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

func RealModelCounts(c *gin.Context) {
	rows, err := repos().Stats.ListRealModelCallsDesc(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}

	results := make([]Count, 0, len(rows))
	for _, row := range rows {
		results = append(results, Count{
			Model:            row.Name,
			Calls:            row.Calls,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			TotalTokens:      row.TotalTokens,
		})
	}

	httpresp.Success(c, results)
}
