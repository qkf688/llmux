package chat

import (
	"testing"

	"github.com/atopos31/llmio/consts"
)

func TestRegistry_GetBeforer_KnownStyles(t *testing.T) {
	for _, style := range []string{consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic} {
		b, err := GetBeforer(style)
		if err != nil {
			t.Errorf("GetBeforer(%q) error: %v", style, err)
		}
		if b == nil {
			t.Errorf("GetBeforer(%q) returned nil", style)
		}
	}
}

func TestRegistry_GetProcesser_KnownStyles(t *testing.T) {
	for _, style := range []string{consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic} {
		p, err := GetProcesser(style)
		if err != nil {
			t.Errorf("GetProcesser(%q) error: %v", style, err)
		}
		if p == nil {
			t.Errorf("GetProcesser(%q) returned nil", style)
		}
	}
}

func TestRegistry_GetBeforer_UnknownStyle(t *testing.T) {
	_, err := GetBeforer("unknown")
	if err == nil {
		t.Fatal("expected error for unknown beforer style")
	}
}

func TestRegistry_GetProcesser_UnknownStyle(t *testing.T) {
	_, err := GetProcesser("unknown")
	if err == nil {
		t.Fatal("expected error for unknown processer style")
	}
}

func TestRegistry_RegisterBeforer_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.RegisterBeforer("x", func([]byte) (*Before, error) { return nil, nil })
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic on duplicate beforer registration")
		}
	}()
	r.RegisterBeforer("x", func([]byte) (*Before, error) { return nil, nil })
}

func TestRegistry_RegisterProcesser_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.RegisterProcesser("x", nil)
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic on duplicate processer registration")
		}
	}()
	r.RegisterProcesser("x", nil)
}
