package chat

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

type singleProviderAttemptInput struct {
	Ctx               context.Context
	Start             time.Time
	Style             string
	Before            Before
	RealModelName     string
	ReqMeta           models.ReqMeta
	IOLog             bool
	Retry             int
	Provider          models.Provider
	ModelWithProvider models.ModelWithProvider
	ChatModel         providers.Provider
	Client            *http.Client
}

type singleProviderAttemptResult struct {
	Response       *http.Response
	LogID          uint
	Success        bool
	FatalErr       error
	RemoveWeight   bool
	RemovePriority bool
	ReduceWeight   bool

	// RawAccumulator 流式响应时累积上游原始 SSE 字节流，供 RecordLog 写入 RawResponseBody。
	// 非流式或未开启记录时为 nil。goroutine 写、流结束后读，无并发。
	RawAccumulator *strings.Builder
}

type requestLogSnapshot struct {
	RequestHeadersJSON []byte
	RequestBodyStr     string
	RawRequestBodyStr  string
}
