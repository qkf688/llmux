package testapi

import (
	"errors"
	"testing"

	"github.com/atopos31/llmio/consts"
)

func TestBuildTestBody(t *testing.T) {
	tests := []struct {
		name      string
		style     string
		expectErr bool
	}{
		{name: "openai", style: consts.StyleOpenAI, expectErr: false},
		{name: "anthropic", style: consts.StyleAnthropic, expectErr: false},
		{name: "openai responses", style: consts.StyleOpenAIRes, expectErr: false},
		{name: "invalid style", style: "unknown-style", expectErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, err := buildTestBody(tc.style)
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
		name      string
		style     string
		expectErr bool
	}{
		{name: "openai", style: consts.StyleOpenAI, expectErr: false},
		{name: "anthropic", style: consts.StyleAnthropic, expectErr: false},
		{name: "openai responses", style: consts.StyleOpenAIRes, expectErr: false},
		{name: "invalid style", style: "unknown-style", expectErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, err := buildStructuredOutputTestBody(tc.style)
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
