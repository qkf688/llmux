package logs

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
)

type chatLogEnrichResult struct {
	isVirtualModel      bool
	hasFormatConversion bool
	sourceFormat        string
	targetFormat        string
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

// enrichChatLogs 多表只读富化：VirtualModel / Provider 查询暂留此处（plan 白名单例外）。
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
			isVirtualModel:      isVirtual,
			hasFormatConversion: hasFormatConversion,
			sourceFormat:        log.Style,
			targetFormat:        providerType,
		}

		enrichedLogs = append(enrichedLogs, buildEnrichedChatLog(log, enrich, includeRaw))
	}

	return enrichedLogs
}