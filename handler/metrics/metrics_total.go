package metrics

import (
	"errors"

	"github.com/atopos31/llmio/httpresp"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func MetricsTotal(c *gin.Context) {
	total, err := repos().Stats.GetTotal(c.Request.Context())
	if err != nil {
		// 尚无任何请求时统计行不存在，按零值返回而非报错
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.Success(c, MetricsRes{Reqs: 0, Tokens: 0})
			return
		}
		httpresp.InternalServerError(c, "Failed to query total metrics: "+err.Error())
		return
	}

	httpresp.Success(c, MetricsRes{Reqs: total.Reqs, Tokens: total.Tokens})
}
