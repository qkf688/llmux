package common

import (
	"net/http"
	"net/textproto"
	"strings"
	"testing"
)

func TestMaskSensitiveLogHeaders_StaticDenyList(t *testing.T) {
	h := http.Header{
		"Authorization":       {"Bearer sk-secret"},
		"Cookie":              {"session=abc"},
		"X-Api-Key":           {"sk-key"},
		"Proxy-Authorization": {"Basic abc"},
		"Set-Cookie":          {"sid=xyz"},
		"X-Custom":            {"trace-1"},
	}
	got := MaskSensitiveLogHeaders(h)
	for _, key := range []string{"Authorization", "Cookie", "X-Api-Key", "Proxy-Authorization", "Set-Cookie"} {
		if got[key][0] != SensitiveHeaderMask {
			t.Fatalf("header %s not masked: %v", key, got[key])
		}
	}
	if got["X-Custom"][0] != "trace-1" {
		t.Fatalf("non-sensitive header value changed: %v", got["X-Custom"])
	}
	// 原入参不被污染（克隆语义）。
	if h["Authorization"][0] != "Bearer sk-secret" {
		t.Fatalf("input header mutated in place: %v", h["Authorization"])
	}
}

func TestMaskHeadersWithDenyList_ExtraKeys(t *testing.T) {
	h := http.Header{
		"Authorization": {"Bearer sk-secret"},
		"X-Relay-Token": {"relay-sk-xxx"},
		"X-Region":      {"us-east"},
		textproto.CanonicalMIMEHeaderKey("x-org-id"): {"org-42"},
	}
	got := MaskHeadersWithDenyList(h, []string{"X-Relay-Token", "x-org-id"})
	if got["Authorization"][0] != SensitiveHeaderMask {
		t.Fatalf("static deny-list key not masked: %v", got["Authorization"])
	}
	if got["X-Relay-Token"][0] != SensitiveHeaderMask {
		t.Fatalf("extra key not masked: %v", got["X-Relay-Token"])
	}
	if got["X-Org-Id"][0] != SensitiveHeaderMask {
		t.Fatalf("extra key (non-canonical input) not masked: %v", got["X-Org-Id"])
	}
	if got["X-Region"][0] != "us-east" {
		t.Fatalf("non-listed key value should be preserved: %v", got["X-Region"])
	}
	// 原入参不被污染。
	if h["X-Relay-Token"][0] != "relay-sk-xxx" {
		t.Fatalf("input header mutated in place: %v", h["X-Relay-Token"])
	}
}

func TestMaskHeadersWithDenyList_NilHeader(t *testing.T) {
	got := MaskHeadersWithDenyList(nil, []string{"X-Relay"})
	if got == nil || len(got) != 0 {
		t.Fatalf("nil header should yield empty non-nil map, got %v", got)
	}
}

func TestMaskHeadersWithDenyList_EmptyExtraKeys(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer x"}, "X-A": {"1"}}
	got := MaskHeadersWithDenyList(h, nil)
	if got["Authorization"][0] != SensitiveHeaderMask || got["X-A"][0] != "1" {
		t.Fatalf("empty extra keys should behave like MaskSensitiveLogHeaders: %v", got)
	}
	if !strings.HasPrefix(got["Authorization"][0], "***") && got["Authorization"][0] != SensitiveHeaderMask {
		t.Fatalf("unexpected mask value: %q", got["Authorization"][0])
	}
}
