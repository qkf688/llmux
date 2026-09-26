package chat

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/models"
)

// RecordRetryLog 消费 retryLog 通道：把「上游请求构建期」失败日志落库。
// 职责仅剩留痕——权重/优先级衰减曾在此按 log.Status=="error" 无条件执行，绕过了结算
// 判据（Stage 24：调用方撤走时不该罚供应商，且失败发生在上游被触达之前），现已移回
// 产出侧经 beginFailureSettlement 判定；本函数在 bgtask ctx 上跑，只做持久化。
func RecordRetryLog(ctx context.Context, retryLog chan models.ChatLog) {
	for log := range retryLog {
		if _, err := SaveChatLog(ctx, log); err != nil {
			slog.Error("save chat log error", "error", err)
		}
	}
}
