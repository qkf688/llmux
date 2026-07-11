package logs

import (
	"errors"
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/handler/httpx"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetRequestLogs 获取最近的请求日志（支持分页和筛选）。
func GetRequestLogs(c *gin.Context) {
	page, pageSize, ok := httpx.ParsePaginationStrict(c)
	if !ok {
		return
	}

	includeRaw := strings.EqualFold(strings.TrimSpace(c.Query("include_raw")), "true")
	filter := repository.ChatLogFilter{
		ProviderName: c.Query("provider_name"),
		Name:         c.Query("name"),
		Status:       c.Query("status"),
		Style:        c.Query("style"),
		UserAgent:    c.Query("user_agent"),
	}

	list, err := repos().ChatLog.List(c.Request.Context(), repository.ChatLogListOptions{
		Filter:     filter,
		IncludeRaw: includeRaw,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		common.InternalServerError(c, "Failed to query logs: "+err.Error())
		return
	}

	enrichedLogs := enrichChatLogs(c.Request.Context(), list.Logs, includeRaw)

	common.Success(c, map[string]any{
		"data":      enrichedLogs,
		"total":     list.Total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (list.Total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// GetRequestLogDetail 获取单条请求日志详情。
func GetRequestLogDetail(c *gin.Context) {
	id, ok := httpx.ParseUintParam(c, "id")
	if !ok {
		return
	}

	log, err := repos().ChatLog.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.NotFound(c, "Log not found")
			return
		}
		common.InternalServerError(c, "Failed to query log: "+err.Error())
		return
	}

	enriched := enrichChatLogs(c.Request.Context(), []models.ChatLog{*log}, true)
	if len(enriched) == 0 {
		common.NotFound(c, "Log not found")
		return
	}
	common.Success(c, enriched[0])
}