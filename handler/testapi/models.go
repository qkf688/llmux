package testapi

import (
	"context"
	"net/http"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

type ProviderModelTestRequest struct {
	Model string `json:"model"`
}

type ChatModel struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Model           string            `json:"model"`
	Config          string            `json:"config"`
	Proxy           string            `json:"proxy"`
	WithHeader      *bool             `json:"with_header,omitempty"`
	CustomerHeaders map[string]string `json:"customer_headers,omitempty"`
}

func FindChatModel(ctx context.Context, id string) (*ChatModel, error) {
	modelWithProvider, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		return nil, err
	}

	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", modelWithProvider.ProviderID).First(ctx)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		Name:            provider.Name,
		Type:            provider.Type,
		Model:           modelWithProvider.ProviderModel,
		Config:          provider.Config,
		Proxy:           provider.Proxy,
		WithHeader:      modelWithProvider.WithHeader,
		CustomerHeaders: modelWithProvider.CustomerHeaders,
	}, nil
}

func BuildTestHeaders(source http.Header, withHeader *bool, customHeaders map[string]string) http.Header {
	header := http.Header{}

	if withHeader != nil && *withHeader {
		header = source.Clone()
	}

	for key, value := range customHeaders {
		header.Set(key, value)
	}

	return header
}
