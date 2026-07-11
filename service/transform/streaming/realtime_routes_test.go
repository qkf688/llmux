package streaming

import "testing"

func TestRealtimeRoute_Registered(t *testing.T) {
	pairs := [][2]string{
		{"anthropic", "openai-res"},
		{"openai-res", "anthropic"},
		{"openai", "openai-res"},
		{"openai-res", "openai"},
	}
	for _, p := range pairs {
		if _, ok := lookupRealtimeRoute(p[0], p[1]); !ok {
			t.Errorf("route %s -> %s not registered", p[0], p[1])
		}
	}
}

func TestRealtimeRoute_Unknown(t *testing.T) {
	if _, ok := lookupRealtimeRoute("unknown", "openai"); ok {
		t.Fatal("expected unknown route missing")
	}
}

func TestRealtimeRoute_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate route")
		}
	}()
	RegisterRealtimeRoute("openai", "openai-res", func(*realtimeStreamState, string) error { return nil })
}