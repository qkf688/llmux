package logs

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

type chatLogEnrichResult struct {
	isVirtualModel      bool
	hasFormatConversion bool
	sourceFormat        string
	targetFormat        string
}

// enrichChatLogs 只读富化：虚拟模型名匹配（VirtualModel 查询暂留此处，plan 白名单例外）。
func enrichChatLogs(ctx context.Context, logs []models.ChatLog, includeRaw bool) []chatLogResponse {
	if len(logs) == 0 {
		return []chatLogResponse{}
	}

	nameSet := make(map[string]struct{}, len(logs))
	for _, log := range logs {
		if log.Name != "" {
			nameSet[log.Name] = struct{}{}
		}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
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

	enrichedLogs := make([]chatLogResponse, 0, len(logs))
	for _, log := range logs {
		_, isVirtual := virtualNameSet[log.Name]

		// 是否真的发生了格式转换，取决于两端的**协议形状**是否不同。出站侧取
		// **命中端点协议**（ChatLog.EndpointProtocol，S3-3 起建行填充）——与 chat
		// 链路的透传判定取数点同源（chat_attempt.go 两侧均按选中端点协议），按
		// Provider.Type 回放会对多协议供应商（type=openai + anthropic 端点）给出
		// 与请求实际行为相反的转换标签。
		// EndpointProtocol 为空（S3-3 之前的存量行）时判无转换：宁少报，不回退
		// Provider.Type 猜测——猜测对多协议供应商必然产生假标签。
		clientFormat, clientFormatOK := consts.WireFormatOfStyle(consts.Style(log.Style))
		upstreamFormat, upstreamFormatOK := consts.WireFormatOfProtocol(consts.Protocol(log.EndpointProtocol))
		hasFormatConversion := clientFormatOK && upstreamFormatOK && clientFormat != upstreamFormat
		enrich := chatLogEnrichResult{
			isVirtualModel:      isVirtual,
			hasFormatConversion: hasFormatConversion,
			sourceFormat:        string(clientFormat),
			targetFormat:        string(upstreamFormat),
		}

		enrichedLogs = append(enrichedLogs, buildChatLogResponse(log, enrich, includeRaw))
	}

	return enrichedLogs
}
