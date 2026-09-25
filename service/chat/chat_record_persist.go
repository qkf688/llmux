package chat

import (
	"context"
	"log/slog"
	"strings"

	"github.com/qkf688/llmux/common"
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
	// 写库不陪葬请求 ctx（Stage 17 收口延伸，审查回批）：RecordLog 生产调用方
	//（handler/v1）传 Background 本就安全，此处派生是防线——未来任何调用方传
	// 请求 ctx，终态回写与 ChatIO 写入都不会静默丢失；err 传播语义保持不变
	//（与 updateChatLogByID 的「仅 slog 不返回」不同，本路径失败要中止后处理）。
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	defer cancel()

	if opts.ResponseBody {
		applyResponseBodyToLog(writeCtx, &logUpdate, output)
	}

	if _, err := repos().ChatLog.UpdateByID(writeCtx, in.LogID, logUpdate); err != nil {
		slog.Error("failed to update log", "log_id", in.LogID, "error", err)
		return err
	}

	if in.IOLog {
		if err := createChatIO(writeCtx, in.LogID, in.Before.raw, capOutputUnion(writeCtx, output)); err != nil {
			return err
		}
	}
	return nil
}

// capOutputUnion ChatIO 落库前的体积保险闸（Stage 23）：processer 侧已由
// outputBudgetAccumulator 收口，这里是防线——未来调用方直接传大数组（绕过
// processer）时 ChatIO 不会被重新撑开。非流式 OfString 与流式数组同一预算。
func capOutputUnion(ctx context.Context, output *models.OutputUnion) *models.OutputUnion {
	if output == nil {
		return nil
	}
	limit := getResponseLogMaxBytes(ctx)

	total := len(output.OfString)
	if total == 0 {
		for _, c := range output.OfStringArray {
			total += len(c)
		}
	}
	if total <= limit {
		return output
	}

	half := (limit - responseLogMarkerReserve) / 2
	capped := models.OutputUnion{}
	var body string
	if output.OfString != "" {
		body = output.OfString
	} else {
		var b strings.Builder
		for _, chunk := range output.OfStringArray {
			b.WriteString(chunk)
			b.WriteByte('\n')
		}
		body = b.String()
	}
	out := common.TruncateHeadTailUTF8(body, half, (limit-responseLogMarkerReserve)-half)
	if output.OfString != "" {
		capped.OfString = out
	} else {
		capped.OfStringArray = []string{out}
	}
	return &capped
}

// applyResponseBodyToLog 将 processer 产出的输出联合体拼装为字符串，填入 log.ResponseBody。
// 非流式：OfString 即完整响应体；流式：OfStringArray 按 chunk 顺序拼接（每个 chunk 占一行）。
// 开启 ResponseBody 记录时始终赋值（含空串），与拆分前闭包语义一致。
// Stage 23：落库文本受 getResponseLogMaxBytes 上限约束（流式/非流式同一把刀），
// 超限存「头段 + 统一标记 + 尾段」（common.TruncateHeadTailUTF8 单源）。
func applyResponseBodyToLog(ctx context.Context, log *models.ChatLog, output *models.OutputUnion) {
	if output == nil {
		log.ResponseBody = ""
		return
	}
	if output.OfString != "" {
		log.ResponseBody = truncateResponseBody(ctx, output.OfString)
		return
	}
	if len(output.OfStringArray) > 0 {
		body := ""
		for _, chunk := range output.OfStringArray {
			body += chunk + "\n"
		}
		log.ResponseBody = truncateResponseBody(ctx, body)
		return
	}
	log.ResponseBody = ""
}

// responseLogMarkerReserve 头尾标记的最大字节预算：基准文案 101 字节（0 值），
// 四个数值最多 5 位（16384 上限内至多 5 位数）各 +4 字节，取 120 留余量。
// 两侧保留字节数的预算 = limit − 本值；即使数值位数到顶，总长仍 ≤ limit。
const responseLogMarkerReserve = 120

// truncateResponseBody 回复日志单条上限的单一收口：超限头尾保留（收尾事件/结尾
// 错误信息对排障与头部同等关键，纯尾切会丢掉它们）。标记字节计入预算之内——
// 「上限」对库体积的承诺是硬的，总长必须 ≤ limit。
func truncateResponseBody(ctx context.Context, body string) string {
	limit := getResponseLogMaxBytes(ctx)
	if len(body) <= limit {
		return body
	}
	budget := limit - responseLogMarkerReserve
	return common.TruncateHeadTailUTF8(body, budget/2, budget-budget/2)
}

// createChatIO 写入一条 ChatIO 记录（原始输入 + 转换后输出联合体）。
func createChatIO(ctx context.Context, logID uint, input []byte, output *models.OutputUnion) error {
	if output == nil {
		output = &models.OutputUnion{}
	}
	if err := repos().ChatIO.Create(ctx, &models.ChatIO{
		Input:       string(input),
		LogID:       logID,
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

	// 清理读写同属日志路径副作用，同样不陪葬请求 ctx（Stage 17 收口延伸）。
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	defer cancel()

	status, err := repos().ChatLog.GetStatus(writeCtx, logID)
	if err != nil || status == "error" {
		return
	}

	if err := repos().ChatLog.ClearRawFields(writeCtx, logID); err != nil {
		slog.Error("failed to clear raw request/response fields", "log_id", logID, "error", err)
	}
}
