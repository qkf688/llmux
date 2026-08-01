package logs

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"gorm.io/gorm"
)

// GetRequestLogDiff 对比单条日志的原始请求体与转换后请求体，返回结构化差异报告。
func GetRequestLogDiff(c *gin.Context) {
	id, ok := httpx.ParseUintParam(c, "id")
	if !ok {
		return
	}

	log, err := repos().ChatLog.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Log not found")
			return
		}
		httpresp.InternalServerError(c, "Failed to query log: "+err.Error())
		return
	}

	if log.RawRequestBody == "" && log.RequestBody == "" {
		httpresp.BadRequest(c, "Both raw and transformed request bodies are empty")
		return
	}

	rawBody := log.RawRequestBody
	if rawBody == "" {
		rawBody = "{}"
	}
	transformedBody := log.RequestBody
	if transformedBody == "" {
		transformedBody = "{}"
	}

	result, err := common.JSONDiff([]byte(rawBody), []byte(transformedBody))
	if err != nil {
		httpresp.BadRequest(c, "Failed to parse request bodies: "+err.Error())
		return
	}

	httpresp.Success(c, result)
}
