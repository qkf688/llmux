package providers

import (
	"testing"
)

func TestRegistry_New_KnownTypes(t *testing.T) {
	tests := []struct {
		name      string
		typ       string
		config    string
		wantProxy string
	}{
		{
			name:      "openai",
			typ:       TypeOpenAI,
			config:    `{"base_url":"http://localhost","api_key":"sk"}`,
			wantProxy: "http://proxy",
		},
		{
			name:      "openai-res",
			typ:       TypeOpenAIRes,
			config:    `{"base_url":"http://localhost","api_key":"sk"}`,
			wantProxy: "http://proxy",
		},
		{
			name:      "anthropic",
			typ:       TypeAnthropic,
			config:    `{"base_url":"http://localhost","api_key":"sk"}`,
			wantProxy: "http://proxy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.typ, tt.config, tt.wantProxy)
			if err != nil {
				t.Fatalf("New(%q) error: %v", tt.typ, err)
			}
			if p == nil {
				t.Fatalf("New(%q) returned nil provider", tt.typ)
			}
			if got := p.GetProxy(); got != tt.wantProxy {
				t.Errorf("GetProxy() = %q, want %q", got, tt.wantProxy)
			}
		})
	}
}

func TestRegistry_New_UnknownType(t *testing.T) {
	_, err := New("unknown", "{}", "")
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}

func TestRegistry_New_ProxyFallback(t *testing.T) {
	p, err := New(TypeOpenAI, `{"base_url":"http://localhost","api_key":"sk","proxy":"http://config-proxy"}`, "")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if got := p.GetProxy(); got != "http://config-proxy" {
		t.Errorf("GetProxy() = %q, want config proxy when external proxy is empty", got)
	}
}

func TestRegistry_Register_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register(TypeOpenAI, func(_, _ string) (Provider, error) { return nil, nil })
}
