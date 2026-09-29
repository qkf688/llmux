package chat

import (
	"context"
	"errors"
	"log/slog"

	"github.com/qkf688/llmux/models"
)

// errRetryLogTaskDiscarded 丢弃窗口的收手信号（Stage 35）：重试日志的落库任务被
// 关机丢弃，重试循环不再发起尝试——没有消费者的 retryLog 投满即挂死，且进程正在
// 收尾、重试结果无处交付。对外经 retryLoopAborted 走「直接终止请求」的失败收尾。
var errRetryLogTaskDiscarded = errors.New("retry log task discarded during shutdown")

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
