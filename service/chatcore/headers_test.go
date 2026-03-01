package chatcore

import (
	"net/http"
	"testing"
)

func TestBuildHeaders_WithHeaderAndCustom(t *testing.T) {
	source := http.Header{}
	source.Set("Authorization", "Bearer secret")
	source.Set("X-Api-Key", "abc")
	source.Set("X-Trace", "trace-1")
	source.Set("X-Custom", "from-source")

	got := BuildHeaders(source, true, map[string]string{
		"X-Custom": "from-custom",
		"X-New":    "new-value",
	}, true)

	if got.Get("Authorization") != "" {
		t.Fatalf("expected Authorization removed, got %q", got.Get("Authorization"))
	}
	if got.Get("X-Api-Key") != "" {
		t.Fatalf("expected X-Api-Key removed, got %q", got.Get("X-Api-Key"))
	}
	if got.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("expected stream header set, got %q", got.Get("X-Accel-Buffering"))
	}
	if got.Get("X-Trace") != "trace-1" {
		t.Fatalf("expected cloned header X-Trace, got %q", got.Get("X-Trace"))
	}
	if got.Get("X-Custom") != "from-custom" {
		t.Fatalf("expected custom header override, got %q", got.Get("X-Custom"))
	}
	if got.Get("X-New") != "new-value" {
		t.Fatalf("expected custom header X-New, got %q", got.Get("X-New"))
	}

	// source must stay unchanged.
	if source.Get("Authorization") != "Bearer secret" {
		t.Fatalf("expected source header unchanged, got %q", source.Get("Authorization"))
	}
}

func TestBuildHeaders_WithoutSourceClone(t *testing.T) {
	source := http.Header{}
	source.Set("X-Trace", "trace-1")

	got := BuildHeaders(source, false, map[string]string{
		"X-Custom": "from-custom",
	}, false)

	if got.Get("X-Trace") != "" {
		t.Fatalf("expected source header not cloned, got %q", got.Get("X-Trace"))
	}
	if got.Get("X-Custom") != "from-custom" {
		t.Fatalf("expected custom header set, got %q", got.Get("X-Custom"))
	}
}
