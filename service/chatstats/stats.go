// Package chatstats 负责把一次请求的结果落到各维度统计表。
// 本包只表达「一次请求要记哪几张表」的业务组合，具体写入形状由 repository.StatsRepo 承担。
package chatstats

import (
	"context"
	"time"

	"github.com/qkf688/llmux/repository"
)

// RecordRequestStats 累加请求次数到时间维度统计（total/daily/hourly）。模型
// 维度的虚拟→真实聚合改走 chat_logs 行级 GROUP BY（详见
// repository.ChatLogRepo.GroupRoutingDesc），不再维护 StatsModelTotal 累计表。
// modelName 参数保留以保持调用方兼容；不再写入任何表。
func RecordRequestStats(ctx context.Context, at time.Time, _ string) error {
	return repos().Stats.AddTimeBased(ctx, at, repository.StatsFieldReqs, 1)
}

// RecordRealModelRequestStats 累加真实模型维度 calls。
func RecordRealModelRequestStats(ctx context.Context, realModelName string) error {
	return repos().Stats.IncRealModelCalls(ctx, realModelName)
}

// RecordRealModelTokenStats 累加真实模型维度 token 三列。
// 与 RecordRealModelRequestStats 分属不同时点：本函数只在响应结束、usage
// 归集完成后调用（调用方 chat.RecordLog），而 calls 在建行时刻就记了。
// realModelName 必须与建行那次同源（RealModelName 空则回落 Name），否则
// 同一个模型会裂成两行各自计数。
func RecordRealModelTokenStats(ctx context.Context, realModelName string, prompt, completion, total int64) error {
	return repos().Stats.AddRealModelTokens(ctx, repository.RealModelTokenDelta{
		Name:             realModelName,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
	})
}

// RecordProviderStats 累加供应商维度请求/成功失败/token/响应时间。
func RecordProviderStats(ctx context.Context, providerName string, success bool, responseTimeMs int64, tokens int64) error {
	return repos().Stats.AddProviderStats(ctx, repository.ProviderStatsDelta{
		ProviderName:   providerName,
		Success:        success,
		ResponseTimeMs: responseTimeMs,
		Tokens:         tokens,
	})
}

// RecordTokenStats 累加 token 次数（total/daily/hourly）。
func RecordTokenStats(ctx context.Context, at time.Time, tokens int64) error {
	return repos().Stats.AddTimeBased(ctx, at, repository.StatsFieldTokens, tokens)
}
