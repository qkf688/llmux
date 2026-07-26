package chat

import (
	"context"
	"log/slog"
)

// maybeClearRawOnSuccess 实现「仅保留错误日志的原始请求响应」策略：
// 若 errorsOnly=true 且 rawLogEnabled，则查询该日志当前状态；
// 状态非 error（即成功）时清空 raw 字段，避免成功日志长期占用存储。
// 错误状态下保留 raw 字段以便排查。
func maybeClearRawOnSuccess(ctx context.Context, logId uint, errorsOnly bool, rawLogEnabled bool) {
	if !errorsOnly || !rawLogEnabled || logId == 0 {
		return
	}

	status, err := repos().ChatLog.GetStatus(ctx, logId)
	if err != nil || status == "error" {
		return
	}

	if err := repos().ChatLog.ClearRawFields(ctx, logId); err != nil {
		slog.Error("failed to clear raw request/response fields", "log_id", logId, "error", err)
	}
}
