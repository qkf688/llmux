package transform

import (
	"context"
	"fmt"

	"github.com/atopos31/llmio/models"
)

const defaultFormatType = "openai"

type FormatAdapter interface {
	ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error)
	FromUnified(unified *models.UnifiedRequest) ([]byte, error)
	ParseResponse(body []byte) (*models.UnifiedResponse, error)
	FormatResponse(unified *models.UnifiedResponse) ([]byte, error)
}

var formatAdapters = map[string]FormatAdapter{}

func RegisterAdapter(name string, adapter FormatAdapter) {
	if name == "" {
		panic("transform adapter name must not be empty")
	}
	if adapter == nil {
		panic(fmt.Sprintf("transform adapter %q must not be nil", name))
	}
	if _, exists := formatAdapters[name]; exists {
		panic(fmt.Sprintf("transform adapter %q is already registered", name))
	}
	formatAdapters[name] = adapter
}

func getAdapterOrDefault(name string) (FormatAdapter, error) {
	if adapter, ok := formatAdapters[name]; ok {
		return adapter, nil
	}
	if adapter, ok := formatAdapters[defaultFormatType]; ok {
		return adapter, nil
	}
	return nil, fmt.Errorf("default transform adapter %q is not registered", defaultFormatType)
}
