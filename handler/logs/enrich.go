package logs

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/models"
)

type chatLogEnrichResult struct {
	isVirtualModel      bool
	hasFormatConversion bool
	sourceFormat        string
	targetFormat        string
}

// enrichChatLogs 多表只读富化：VirtualModel / Provider 查询暂留此处（plan 白名单例外）。
func enrichChatLogs(ctx context.Context, logs []models.ChatLog, includeRaw bool) []chatLogResponse {
	if len(logs) == 0 {
		return []chatLogResponse{}
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

	enrichedLogs := make([]chatLogResponse, 0, len(logs))
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

		enrichedLogs = append(enrichedLogs, buildChatLogResponse(log, enrich, includeRaw))
	}

	return enrichedLogs
}
