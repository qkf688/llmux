package chat

import (
	"context"
	"net/http"
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
	// Model 当前尝试关联所属的真实模型；用于 SupportsThinkingResolved 的 model 继承来源。
	Model     *models.Model
	ChatModel providers.Provider
	Client    *http.Client
}

type singleProviderAttemptResult struct {
	Response       *http.Response
	LogID          uint
	Success        bool
	FatalErr       error
	RemoveWeight   bool
	RemovePriority bool
	ReduceWeight   bool

	// SideChannel 承载转换旁路产物：上游原始 SSE 累积体（供 RawResponseBody）
	// 与上游原始 usage（供落库统计）。直通路径无转换层可旁路，故为 nil。
	// 转换 goroutine 写、流结束后读，内部自带互斥。
	SideChannel *models.TransformSideChannel
}

type requestLogSnapshot struct {
	RequestHeadersJSON []byte
	RequestBodyStr     string
	RawRequestBodyStr  string
}
