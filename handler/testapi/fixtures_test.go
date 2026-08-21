package testapi

import (
	"errors"
	"testing"

	"github.com/qkf688/llmux/providers"
)

func TestBuildTestBody(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		expectErr    bool
	}{
		{name: "openai", providerType: providers.TypeOpenAI, expectErr: false},
		{name: "anthropic", providerType: providers.TypeAnthropic, expectErr: false},
		{name: "openai responses", providerType: providers.TypeOpenAIRes, expectErr: false},
		{name: "invalid provider type", providerType: "unknown-provider-type", expectErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, err := buildTestBody(tc.providerType)
			if tc.expectErr {
				if !errors.Is(err, errInvalidProviderType) {
					t.Fatalf("expected errInvalidProviderType, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("body should not be empty")
			}
		})
	}
}

func TestBuildStructuredOutputTestBody(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		expectErr    bool
	}{
		{name: "openai", providerType: providers.TypeOpenAI, expectErr: false},
		{name: "anthropic", providerType: providers.TypeAnthropic, expectErr: false},
		{name: "openai responses", providerType: providers.TypeOpenAIRes, expectErr: false},
		{name: "invalid provider type", providerType: "unknown-provider-type", expectErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, err := buildStructuredOutputTestBody(tc.providerType)
			if tc.expectErr {
				if !errors.Is(err, errInvalidProviderType) {
					t.Fatalf("expected errInvalidProviderType, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("body should not be empty")
			}
		})
	}
}
