package providers

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

func TestMetadata_RegisteredTypes(t *testing.T) {
	types := []string{consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic}
	for _, typ := range types {
		m, ok := MetadataOf(typ)
		if !ok {
			t.Fatalf("MetadataOf(%q) not found", typ)
		}
		if m.Type != typ {
			t.Errorf("Type = %q, want %q", m.Type, typ)
		}
		if m.ConfigTemplate == "" {
			t.Errorf("%s: empty ConfigTemplate", typ)
		}
		if len(m.TestBody) == 0 {
			t.Errorf("%s: empty TestBody", typ)
		}
		if len(m.StructuredBody) == 0 {
			t.Errorf("%s: empty StructuredBody", typ)
		}
		if len(m.HealthCheckBody) == 0 {
			t.Errorf("%s: empty HealthCheckBody", typ)
		}
	}
}

func TestMetadata_UnknownType(t *testing.T) {
	if _, ok := MetadataOf("unknown-type"); ok {
		t.Fatal("expected false for unknown type")
	}
}

func TestMetadata_AllOrder(t *testing.T) {
	all := AllMetadata()
	if len(all) < 3 {
		t.Fatalf("AllMetadata len = %d, want >= 3", len(all))
	}
	// 稳定顺序：openai → openai-res → anthropic（与旧 template 切片一致）
	want := []string{consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic}
	got := make([]string, 0, 3)
	for _, m := range all {
		switch m.Type {
		case consts.StyleOpenAI, consts.StyleOpenAIRes, consts.StyleAnthropic:
			got = append(got, m.Type)
		}
	}
	if len(got) != 3 {
		t.Fatalf("core types count = %d, want 3", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMetadata_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate metadata registration")
		}
	}()
	RegisterMetadata(Metadata{Type: consts.StyleOpenAI, ConfigTemplate: "{}"})
}
