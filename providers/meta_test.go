package providers

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

func TestMetadata_RegisteredTypes(t *testing.T) {
	// provider type → 上游 body 形状。这张表是 transform 侧选适配器的唯一依据，声明错会让请求体按错误协议构建。
	wantFormats := map[string]consts.WireFormat{
		TypeOpenAI:    consts.FormatOpenAIChat,
		TypeOpenAIRes: consts.FormatOpenAIResponses,
		TypeAnthropic: consts.FormatAnthropic,
	}
	for typ, wantFormat := range wantFormats {
		m, ok := MetadataOf(typ)
		if !ok {
			t.Fatalf("MetadataOf(%q) not found", typ)
		}
		if m.Type != typ {
			t.Errorf("Type = %q, want %q", m.Type, typ)
		}
		if m.WireFormat != wantFormat {
			t.Errorf("%s: WireFormat = %q, want %q", typ, m.WireFormat, wantFormat)
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

// TestWireFormatOf 锁定「未注册 type 不兜底」——兜底会让漏注册的 provider 静默按 OpenAI 形状发请求。
func TestWireFormatOf(t *testing.T) {
	got, ok := WireFormatOf(TypeAnthropic)
	if !ok {
		t.Fatal("WireFormatOf(anthropic) not found")
	}
	if got != consts.FormatAnthropic {
		t.Errorf("WireFormatOf(anthropic) = %q, want %q", got, consts.FormatAnthropic)
	}

	if got, ok := WireFormatOf("unknown-type"); ok || got != "" {
		t.Errorf("WireFormatOf(unknown-type) = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestMetadata_AllOrder(t *testing.T) {
	all := AllMetadata()
	if len(all) < 3 {
		t.Fatalf("AllMetadata len = %d, want >= 3", len(all))
	}
	// 稳定顺序：openai → openai-res → anthropic（与旧 template 切片一致）
	want := []string{TypeOpenAI, TypeOpenAIRes, TypeAnthropic}
	got := make([]string, 0, 3)
	for _, m := range all {
		switch m.Type {
		case TypeOpenAI, TypeOpenAIRes, TypeAnthropic:
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
	// 必须带 WireFormat，否则会先撞上「未声明形状」的 panic，测不到重复注册这条。
	RegisterMetadata(Metadata{Type: TypeOpenAI, WireFormat: consts.FormatOpenAIChat, ConfigTemplate: "{}"})
}

func TestMetadata_MissingWireFormatPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when metadata declares no wire format")
		}
	}()
	RegisterMetadata(Metadata{Type: "provider-without-wire-format", ConfigTemplate: "{}"})
}
