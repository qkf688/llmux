package logs

import (
	"errors"
	"strconv"
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetRequestLogs 获取最近的请求日志（支持分页和筛选）。
func GetRequestLogs(c *gin.Context) {
	pageStr := c.Query("page")
	page := 1
	if pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err != nil || parsedPage < 1 {
			common.BadRequest(c, "Invalid page parameter")
			return
		}
		page = parsedPage
	}

	pageSizeStr := c.Query("page_size")
	pageSize := 20
	if pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			common.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return
		}
		pageSize = parsedPageSize
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
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	log, err := repos().ChatLog.Get(c.Request.Context(), uint(id))
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