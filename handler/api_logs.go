package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func applyChatLogFilters(
	query *gorm.DB,
	providerName string,
	name string,
	status string,
	style string,
	userAgent string,
) *gorm.DB {
	providerName = strings.TrimSpace(providerName)
	name = strings.TrimSpace(name)
	status = strings.TrimSpace(status)
	style = strings.TrimSpace(style)
	userAgent = strings.TrimSpace(userAgent)

	if providerName != "" {
		query = query.Where("provider_name = ?", providerName)
	}

	if name != "" {
		query = query.Where("name = ?", name)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if style != "" {
		query = query.Where("style = ?", style)
	}

	if userAgent != "" {
		query = query.Where("user_agent = ?", userAgent)
	}

	return query
}

type chatLogEnrichResult struct {
	isVirtualModel     bool
	hasFormatConversion bool
	sourceFormat       string
	targetFormat       string
}

func buildEnrichedChatLog(log models.ChatLog, enrich chatLogEnrichResult, includeRaw bool) map[string]any {
	enrichedLog := map[string]any{
		"ID":                    log.ID,
		"CreatedAt":             log.CreatedAt,
		"Name":                  log.Name,
		"ProviderModel":         log.ProviderModel,
		"ProviderName":          log.ProviderName,
		"Status":                log.Status,
		"Style":                 log.Style,
		"UserAgent":             log.UserAgent,
		"RemoteIP":              log.RemoteIP,
		"Error":                 log.Error,
		"Retry":                 log.Retry,
		"ProxyTime":             log.ProxyTime,
		"FirstChunkTime":        log.FirstChunkTime,
		"ChunkTime":             log.ChunkTime,
		"Tps":                   log.Tps,
		"ChatIO":                log.ChatIO,
		"prompt_tokens":         log.PromptTokens,
		"completion_tokens":     log.CompletionTokens,
		"total_tokens":          log.TotalTokens,
		"prompt_tokens_details": log.PromptTokensDetails,
		"is_virtual_model":      enrich.isVirtualModel,
		"has_format_conversion": enrich.hasFormatConversion,
	}

	if enrich.hasFormatConversion {
		enrichedLog["source_format"] = enrich.sourceFormat
		enrichedLog["target_format"] = enrich.targetFormat
	}

	if includeRaw {
		enrichedLog["RequestHeaders"] = log.RequestHeaders
		enrichedLog["RequestBody"] = log.RequestBody
		enrichedLog["ResponseHeaders"] = log.ResponseHeaders
		enrichedLog["ResponseBody"] = log.ResponseBody
		enrichedLog["RawResponseBody"] = log.RawResponseBody
	}

	return enrichedLog
}

func enrichChatLogs(ctx context.Context, logs []models.ChatLog, includeRaw bool) []map[string]any {
	if len(logs) == 0 {
		return []map[string]any{}
	}

	nameSet := make(map[string]struct{}, len(logs))
	providerNameSet := make(map[string]struct{}, len(logs))
	for _, log := range logs {
		if log.Name != "" {
			nameSet[log.Name] = struct{}{}
		}
		if log.ProviderName != "" {
			providerNameSet[log.ProviderName] = struct{}{}
		}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	providerNames := make([]string, 0, len(providerNameSet))
	for providerName := range providerNameSet {
		providerNames = append(providerNames, providerName)
	}

	virtualNameSet := make(map[string]struct{}, len(names))
	if len(names) > 0 {
		var virtualNames []string
		if err := models.DB.WithContext(ctx).
			Model(&models.VirtualModel{}).
			Where("name IN ? AND enabled = ?", names, true).
			Pluck("name", &virtualNames).
			Error; err != nil {
			slog.Error("failed to query virtual models", "error", err)
		} else {
			for _, name := range virtualNames {
				virtualNameSet[name] = struct{}{}
			}
		}
	}

	providerTypeByName := make(map[string]string, len(providerNames))
	if len(providerNames) > 0 {
		var providers []models.Provider
		if err := models.DB.WithContext(ctx).
			Model(&models.Provider{}).
			Select("name", "type").
			Where("name IN ?", providerNames).
			Find(&providers).Error; err != nil {
			slog.Error("failed to query providers for logs enrichment", "error", err)
		} else {
			for _, provider := range providers {
				providerTypeByName[provider.Name] = provider.Type
			}
		}
	}

	enrichedLogs := make([]map[string]any, 0, len(logs))
	for _, log := range logs {
		_, isVirtual := virtualNameSet[log.Name]
		providerType := providerTypeByName[log.ProviderName]

		hasFormatConversion := providerType != "" && log.Style != "" && log.Style != providerType
		enrich := chatLogEnrichResult{
			isVirtualModel:     isVirtual,
			hasFormatConversion: hasFormatConversion,
			sourceFormat:       log.Style,
			targetFormat:       providerType,
		}

		enrichedLogs = append(enrichedLogs, buildEnrichedChatLog(log, enrich, includeRaw))
	}

	return enrichedLogs
}

// GetRequestLogs 获取最近的请求日志（支持分页和筛选）
func GetRequestLogs(c *gin.Context) {
	// 分页参数
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
	pageSize := 20 // Default page size
	if pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			common.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return
		}
		pageSize = parsedPageSize
	}

	// 筛选参数
	providerName := c.Query("provider_name")
	name := c.Query("name")
	status := c.Query("status")
	style := c.Query("style")
	userAgent := c.Query("user_agent")
	includeRaw := strings.EqualFold(strings.TrimSpace(c.Query("include_raw")), "true")

	// 构建查询条件
	baseQuery := applyChatLogFilters(
		models.DB.Model(&models.ChatLog{}),
		providerName,
		name,
		status,
		style,
		userAgent,
	)

	// 获取总数
	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		common.InternalServerError(c, "Failed to count logs: "+err.Error())
		return
	}

	// 获取分页数据
	var logs []models.ChatLog
	offset := (page - 1) * pageSize
	query := baseQuery
	if !includeRaw {
		query = query.Select(
			"id",
			"created_at",
			"name",
			"provider_model",
			"provider_name",
			"status",
			"style",
			"user_agent",
			"remote_ip",
			"chat_io",
			"error",
			"retry",
			"proxy_time",
			"first_chunk_time",
			"chunk_time",
			"tps",
			"prompt_tokens",
			"completion_tokens",
			"total_tokens",
			"prompt_tokens_details",
			"completion_tokens_details",
		)
	}
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		common.InternalServerError(c, "Failed to query logs: "+err.Error())
		return
	}

	enrichedLogs := enrichChatLogs(c.Request.Context(), logs, includeRaw)

	result := map[string]any{
		"data":      enrichedLogs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	}

	common.Success(c, result)
}

// GetRequestLogDetail 获取单条请求日志详情（包含原始请求/响应大字段）
func GetRequestLogDetail(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var log models.ChatLog
	if err := models.DB.WithContext(c.Request.Context()).First(&log, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.NotFound(c, "Log not found")
			return
		}
		common.InternalServerError(c, "Failed to query log: "+err.Error())
		return
	}

	enriched := enrichChatLogs(c.Request.Context(), []models.ChatLog{log}, true)
	if len(enriched) == 0 {
		common.NotFound(c, "Log not found")
		return
	}

	common.Success(c, enriched[0])
}

// GetChatIO 查询指定日志的输入输出记录
func GetChatIO(c *gin.Context) {
	id := c.Param("id")

	chatIO, err := gorm.G[models.ChatIO](models.DB).Where("log_id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "ChatIO not found")
		return
	}

	common.Success(c, chatIO)
}

// GetUserAgents 获取所有不重复的用户代理种类
func GetUserAgents(c *gin.Context) {
	var userAgents []string

	// 查询所有不重复的非空用户代理
	if err := models.DB.Model(&models.ChatLog{}).
		Where("user_agent IS NOT NULL AND user_agent != ''").
		Distinct("user_agent").
		Pluck("user_agent", &userAgents).
		Error; err != nil {
		common.InternalServerError(c, "Failed to query user agents: "+err.Error())
		return
	}

	common.Success(c, userAgents)
}

// DeleteLog 删除单条日志
func DeleteLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	// 删除对应的ChatIO记录（硬删）
	if err := models.DB.Unscoped().
		Where("log_id = ?", id).
		Delete(&models.ChatIO{}).Error; err != nil {
		slog.Warn("failed to delete chat io record", "log_id", id, "error", err)
	}

	// 删除日志记录（硬删）
	result := models.DB.Unscoped().
		Where("id = ?", id).
		Delete(&models.ChatLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to delete log: "+result.Error.Error())
		return
	}

	if result.RowsAffected == 0 {
		common.NotFound(c, "Log not found")
		return
	}

	common.Success(c, nil)
}

// BatchDeleteLogsRequest 批量删除日志请求结构
type BatchDeleteLogsRequest struct {
	IDs []uint `json:"ids"`
}

// BatchDeleteLogs 批量删除日志
func BatchDeleteLogs(c *gin.Context) {
	var req BatchDeleteLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	// 删除对应的ChatIO记录
	if err := models.DB.Unscoped().
		Where("log_id IN ?", req.IDs).
		Delete(&models.ChatIO{}).Error; err != nil {
		slog.Warn("failed to delete chat io records", "error", err)
	}

	// 删除日志记录
	result := models.DB.Unscoped().
		Where("id IN ?", req.IDs).
		Delete(&models.ChatLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to delete logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

// GetHealthCheckLogs 获取健康检测日志（支持分页和筛选）
func GetHealthCheckLogs(c *gin.Context) {
	// 分页参数
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
	pageSize := 20 // Default page size
	if pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			common.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return
		}
		pageSize = parsedPageSize
	}

	// 筛选参数
	modelProviderID := c.Query("model_provider_id")
	modelName := c.Query("model_name")
	providerName := c.Query("provider_name")
	status := c.Query("status")

	// 构建查询条件
	query := models.DB.Model(&models.HealthCheckLog{})

	if modelProviderID != "" {
		query = query.Where("model_provider_id = ?", modelProviderID)
	}

	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}

	if providerName != "" {
		query = query.Where("provider_name = ?", providerName)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.InternalServerError(c, "Failed to count health check logs: "+err.Error())
		return
	}

	// 获取分页数据
	var logs []models.HealthCheckLog
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		common.InternalServerError(c, "Failed to query health check logs: "+err.Error())
		return
	}

	result := map[string]any{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	}

	common.Success(c, result)
}

// ClearHealthCheckLogs 清空健康检测日志
func ClearHealthCheckLogs(c *gin.Context) {
	result := models.DB.Unscoped().
		Where("1 = 1").
		Delete(&models.HealthCheckLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to clear health check logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

// ClearAllLogs 清空所有日志
func ClearAllLogs(c *gin.Context) {
	// 删除所有ChatIO记录
	if err := models.DB.Unscoped().
		Where("1 = 1").
		Delete(&models.ChatIO{}).Error; err != nil {
		slog.Warn("failed to delete all chat io records", "error", err)
	}

	// 删除所有日志记录
	result := models.DB.Unscoped().
		Where("1 = 1").
		Delete(&models.ChatLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to clear logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

// ClearFilteredLogs 清空满足筛选条件的请求日志（跨分页）
func ClearFilteredLogs(c *gin.Context) {
	// 筛选参数（与 GetRequestLogs 保持一致）
	providerName := strings.TrimSpace(c.Query("provider_name"))
	name := strings.TrimSpace(c.Query("name"))
	status := strings.TrimSpace(c.Query("status"))
	style := strings.TrimSpace(c.Query("style"))
	userAgent := strings.TrimSpace(c.Query("user_agent"))

	if providerName == "" && name == "" && status == "" && style == "" && userAgent == "" {
		common.BadRequest(c, "At least one filter parameter is required")
		return
	}

	tx := models.DB.Begin()
	if tx.Error != nil {
		common.InternalServerError(c, "Failed to start transaction: "+tx.Error.Error())
		return
	}

	rollback := func() {
		_ = tx.Rollback().Error
	}

	subQuery := tx.Model(&models.ChatLog{}).Select("id")
	if providerName != "" {
		subQuery = subQuery.Where("provider_name = ?", providerName)
	}
	if name != "" {
		subQuery = subQuery.Where("name = ?", name)
	}
	if status != "" {
		subQuery = subQuery.Where("status = ?", status)
	}
	if style != "" {
		subQuery = subQuery.Where("style = ?", style)
	}
	if userAgent != "" {
		subQuery = subQuery.Where("user_agent = ?", userAgent)
	}

	// 先删除 ChatIO（硬删）
	if err := tx.Unscoped().
		Where("log_id IN (?)", subQuery).
		Delete(&models.ChatIO{}).Error; err != nil {
		rollback()
		common.InternalServerError(c, "Failed to delete chat io records: "+err.Error())
		return
	}

	// 再删除 ChatLog（硬删）
	result := tx.Unscoped().
		Where("id IN (?)", subQuery).
		Delete(&models.ChatLog{})
	if result.Error != nil {
		rollback()
		common.InternalServerError(c, "Failed to clear logs: "+result.Error.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		rollback()
		common.InternalServerError(c, "Failed to commit transaction: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

// GetModelSyncLogs 获取模型同步日志列表
func GetModelSyncLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 筛选参数
	providerID := c.Query("provider_id")
	status := c.Query("status")
	showUnchanged := c.Query("show_unchanged") == "true"

	// 构建查询
	query := models.DB.Model(&models.ModelSyncLog{}).WithContext(ctx)
	if providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	if status != "" {
		switch status {
		case "success", "error", "unchanged":
			query = query.Where("status = ?", status)
		default:
			common.BadRequest(c, "Invalid status parameter (must be success, error, or unchanged)")
			return
		}
	} else {
		// 默认只显示有更新的记录（除非明确要求显示全部）
		if !showUnchanged {
			query = query.Where("status = ?", "success")
		}
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.InternalServerError(c, "Failed to count logs: "+err.Error())
		return
	}

	// 查询数据
	var logs []models.ModelSyncLog
	offset := (page - 1) * pageSize
	if err := query.Order("synced_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		common.InternalServerError(c, "Failed to get logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"data": logs,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// DeleteModelSyncLogs 批量删除模型同步日志
func DeleteModelSyncLogs(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	result := models.DB.Unscoped().Where("id IN ?", req.IDs).Delete(&models.ModelSyncLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to delete logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

// ClearModelSyncLogs 清空所有模型同步日志
func ClearModelSyncLogs(c *gin.Context) {
	result := models.DB.Unscoped().Where("1 = 1").Delete(&models.ModelSyncLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to clear logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

// ClearModelSyncErrorLogs 清空所有错误模型同步日志
func ClearModelSyncErrorLogs(c *gin.Context) {
	var req struct {
		ProviderIDs []uint `json:"provider_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	query := models.DB.Unscoped().Model(&models.ModelSyncLog{}).Where("status = ?", "error")
	if len(req.ProviderIDs) > 0 {
		query = query.Where("provider_id IN ?", req.ProviderIDs)
	}

	result := query.Delete(&models.ModelSyncLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to clear error logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}
