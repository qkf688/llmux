package service

import (
	"context"
	"io"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/chat"
)

type Before = chat.Before
type Beforer = chat.Beforer

func BeforerOpenAI(data []byte) (*Before, error)    { return chat.BeforerOpenAI(data) }
func BeforerOpenAIRes(data []byte) (*Before, error) { return chat.BeforerOpenAIRes(data) }
func BeforerAnthropic(data []byte) (*Before, error) { return chat.BeforerAnthropic(data) }

func GetBeforer(style string) (Beforer, error) { return chat.GetBeforer(style) }

type Processer = chat.Processer

func ProcesserOpenAI(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
	return chat.ProcesserOpenAI(ctx, pr, stream, start, disablePerformanceTracking, disableTokenCounting)
}

func ProcesserOpenAiRes(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
	return chat.ProcesserOpenAiRes(ctx, pr, stream, start, disablePerformanceTracking, disableTokenCounting)
}

func ProcesserAnthropic(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
	return chat.ProcesserAnthropic(ctx, pr, stream, start, disablePerformanceTracking, disableTokenCounting)
}

func GetProcesser(style string) (Processer, error) { return chat.GetProcesser(style) }

type StreamErrorWriter = chat.StreamErrorWriter

func GetStreamErrorWriter(style string) (StreamErrorWriter, error) {
	return chat.GetStreamErrorWriter(style)
}

type ProvidersWithMeta = chat.ProvidersWithMeta

func ProvidersWithMetaBymodelsName(ctx context.Context, style string, before Before, reqMeta models.ReqMeta) (*ProvidersWithMeta, error) {
	return chat.ProvidersWithMetaBymodelsName(ctx, style, before, reqMeta)
}

type BalanceInput = chat.BalanceInput
type BalanceResult = chat.BalanceResult

func BalanceChat(ctx context.Context, in BalanceInput) (*BalanceResult, error) {
	return chat.BalanceChat(ctx, in)
}

// WithRequestTotalTimeout 给请求挂上网关总超时（Stage 33）：派生带 deadline 的请求 ctx，
// 到点整个请求（含流式输出）被掐断。cancel 必须在响应写完后才调用。
func WithRequestTotalTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return chat.WithRequestTotalTimeout(ctx)
}

type RecordLogInput = chat.RecordLogInput

func RecordLog(ctx context.Context, in RecordLogInput) {
	chat.RecordLog(ctx, in)
}

// MarkUpstreamReader 给 reader 的**读**错误打上「来自上游」方向标记，供 RecordLog 的
// 提交后失败清钉判定区分「上游故障」与「客户端撤走」——io.Copy 的 error 本身不带方向，
// 只能在错误产生处标记。调用点须在 tee 之外（先包 reader 再 TeeReader），见 chat 包注释。
func MarkUpstreamReader(r io.Reader) io.Reader { return chat.MarkUpstreamReader(r) }

// MarkLogUnfinished 给「日志后处理任务被关机丢弃」的日志行写「未能写完」失败痕迹
// （bgtask 丢弃窗口的收尾动作，Stage 35）。丢正文、留痕迹，行不得停在建行占位 success。
func MarkLogUnfinished(ctx context.Context, logID uint, reqStart time.Time) {
	chat.MarkLogUnfinished(ctx, logID, reqStart)
}

func GetStripResponseHeaders(ctx context.Context) bool { return chat.GetStripResponseHeaders(ctx) }
