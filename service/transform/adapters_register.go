package transform

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/anthropic"
	"github.com/atopos31/llmio/service/transform/openai"
	"github.com/atopos31/llmio/service/transform/responses"
)

// funcAdapter 用函数字段实现 FormatAdapter，减少三份薄 struct 样板。
type funcAdapter struct {
	toUnified      func(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error)
	fromUnified    func(unified *models.UnifiedRequest) ([]byte, error)
	parseResponse  func(body []byte) (*models.UnifiedResponse, error)
	formatResponse func(unified *models.UnifiedResponse) ([]byte, error)
}

func (a funcAdapter) ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	return a.toUnified(ctx, rawBody)
}

func (a funcAdapter) FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	return a.fromUnified(unified)
}

func (a funcAdapter) ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	return a.parseResponse(body)
}

func (a funcAdapter) FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	return a.formatResponse(unified)
}

func init() {
	RegisterAdapter("openai", funcAdapter{
		toUnified:      openai.ToUnified,
		fromUnified:    openai.FromUnified,
		parseResponse:  openai.ParseResponse,
		formatResponse: openai.FormatResponse,
	})
	RegisterAdapter("anthropic", funcAdapter{
		toUnified: func(_ context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
			return anthropic.ToUnified(rawBody)
		},
		fromUnified:    anthropic.FromUnified,
		parseResponse:  anthropic.ParseResponse,
		formatResponse: anthropic.FormatResponse,
	})
	RegisterAdapter("openai-res", funcAdapter{
		toUnified:      responses.ToUnified,
		fromUnified:    responses.FromUnified,
		parseResponse:  responses.ParseResponse,
		formatResponse: responses.FormatResponse,
	})
}