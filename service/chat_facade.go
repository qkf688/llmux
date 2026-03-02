package service

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/chat"
)

type Before = chat.Before
type Beforer = chat.Beforer

func BeforerOpenAI(data []byte) (*Before, error)    { return chat.BeforerOpenAI(data) }
func BeforerOpenAIRes(data []byte) (*Before, error) { return chat.BeforerOpenAIRes(data) }
func BeforerAnthropic(data []byte) (*Before, error) { return chat.BeforerAnthropic(data) }

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

type ProvidersWithMeta = chat.ProvidersWithMeta

func ProvidersWithMetaBymodelsName(ctx context.Context, style string, before Before) (*ProvidersWithMeta, error) {
	return chat.ProvidersWithMetaBymodelsName(ctx, style, before)
}

func BalanceChat(ctx context.Context, start time.Time, style string, before Before, providersWithMeta ProvidersWithMeta, reqMeta models.ReqMeta) (*http.Response, uint, error) {
	return chat.BalanceChat(ctx, start, style, before, providersWithMeta, reqMeta)
}

func RecordLog(ctx context.Context, reqStart time.Time, reader io.ReadCloser, processer Processer, logId uint, before Before, ioLog bool) {
	chat.RecordLog(ctx, reqStart, reader, processer, logId, before, ioLog)
}

func GetStripResponseHeaders(ctx context.Context) bool { return chat.GetStripResponseHeaders(ctx) }
