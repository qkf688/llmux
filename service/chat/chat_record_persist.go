package chat

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/models"
)

// persistChatLog 负责响应后处理中的 IO 落库：
//  1. 若开启 ResponseBody 记录，将 output 拼装为字符串写入 logUpdate.ResponseBody；
//  2. 更新 ChatLog 记录（status / usage / timing / response_body 等）；
//  3. 若 in.IOLog=true，写入 ChatIO（输入 + 输出联合体）。
//
// 复用 RecordLogInput 取 LogID / Before / IOLog，另外三个参数是后处理运行时产物，
// 不属于入参快照，故并列传入而非塞进结构体。
// 与 stats.go（统计）各司其职；raw 清理（maybeClearRawOnSuccess）已合入本文件。
func persistChatLog(ctx context.Context, in RecordLogInput, logUpdate models.ChatLog, output *models.OutputUnion, opts models.RawLogOptions) error {
	if opts.ResponseBody {
		applyResponseBodyToLog(&logUpdate, output)
	}

	if _, err := repos().ChatLog.UpdateByID(ctx, in.LogID, logUpdate); err != nil {
		slog.Error("failed to update log", "log_id", in.LogID, "error", err)
		return err
	}

	if in.IOLog {
		if err := createChatIO(ctx, in.LogID, in.Before.raw, output); err != nil {
			return err
		}
	}
	return nil
}

// applyResponseBodyToLog 将 processer 产出的输出联合体拼装为字符串，填入 log.ResponseBody。
// 非流式：OfString 即完整响应体；流式：OfStringArray 按 chunk 顺序拼接（每个 chunk 占一行）。
// 开启 ResponseBody 记录时始终赋值（含空串），与拆分前闭包语义一致。
func applyResponseBodyToLog(log *models.ChatLog, output *models.OutputUnion) {
	if output == nil {
		log.ResponseBody = ""
		return
	}
	if output.OfString != "" {
		log.ResponseBody = output.OfString
		return
	}
	if len(output.OfStringArray) > 0 {
		body := ""
		for _, chunk := range output.OfStringArray {
			body += chunk + "\n"
		}
		log.ResponseBody = body
		return
	}
	log.ResponseBody = ""
}

// createChatIO 写入一条 ChatIO 记录（原始输入 + 转换后输出联合体）。
func createChatIO(ctx context.Context, logID uint, input []byte, output *models.OutputUnion) error {
	if output == nil {
		output = &models.OutputUnion{}
	}
	if err := repos().ChatIO.Create(ctx, &models.ChatIO{
		Input:       string(input),
		LogId:       logID,
		OutputUnion: *output,
	}); err != nil {
		slog.Error("failed to create chat io", "log_id", logID, "error", err)
		return err
	}
	return nil
}

// maybeClearRawOnSuccess 实现「仅保留错误日志的原始请求响应」策略：
// 若 errorsOnly=true 且 rawLogEnabled，则查询该日志当前状态；
// 状态非 error（即成功）时清空 raw 字段，避免成功日志长期占用存储。
// 错误状态下保留 raw 字段以便排查。
func maybeClearRawOnSuccess(ctx context.Context, logID uint, errorsOnly bool, rawLogEnabled bool) {
	if !errorsOnly || !rawLogEnabled || logID == 0 {
		return
	}

	status, err := repos().ChatLog.GetStatus(ctx, logID)
	if err != nil || status == "error" {
		return
	}

	if err := repos().ChatLog.ClearRawFields(ctx, logID); err != nil {
		slog.Error("failed to clear raw request/response fields", "log_id", logID, "error", err)
	}
}
