package testapi

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/providers"
)

func TestExtractVisionText_OpenAI_StringContent(t *testing.T) {
	response := []byte(`{"choices":[{"message":{"content":"The image is red."}}]}`)

	got, err := extractVisionText(consts.FormatOpenAIChat, response)
	if err != nil {
		t.Fatalf("extractVisionText failed: %v", err)
	}
	if got != "The image is red." {
		t.Fatalf("vision text mismatch: got %q", got)
	}
}

func TestExtractVisionText_OpenAI_ArrayContent(t *testing.T) {
	// 部分兼容网关把 content 回成多模态数组：拼接全部 text 部分
	response := []byte(`{"choices":[{"message":{"content":[{"type":"text","text":"这是"},{"type":"text","text":"红色"}]}}]}`)

	got, err := extractVisionText(consts.FormatOpenAIChat, response)
	if err != nil {
		t.Fatalf("extractVisionText failed: %v", err)
	}
	if got != "这是\n红色" {
		t.Fatalf("vision text mismatch: got %q", got)
	}
}

func TestExtractVisionText_OpenAI_NoText(t *testing.T) {
	response := []byte(`{"choices":[{"message":{"content":""}}]}`)

	if _, err := extractVisionText(consts.FormatOpenAIChat, response); !errors.Is(err, errVisionTextNotFound) {
		t.Fatalf("expected errVisionTextNotFound, got %v", err)
	}
}

func TestExtractVisionText_Anthropic(t *testing.T) {
	// thinking 块不进判定文本，只拼接 text 块
	response := []byte(`{"content":[{"type":"thinking","thinking":"looking..."},{"type":"text","text":"The image is red."}]}`)

	got, err := extractVisionText(consts.FormatAnthropic, response)
	if err != nil {
		t.Fatalf("extractVisionText failed: %v", err)
	}
	if got != "The image is red." {
		t.Fatalf("vision text mismatch: got %q", got)
	}
}

func TestExtractVisionText_Anthropic_NoText(t *testing.T) {
	response := []byte(`{"content":[{"type":"thinking","thinking":"looking..."}]}`)

	if _, err := extractVisionText(consts.FormatAnthropic, response); !errors.Is(err, errVisionTextNotFound) {
		t.Fatalf("expected errVisionTextNotFound, got %v", err)
	}
}

func TestExtractVisionText_OpenAIRes(t *testing.T) {
	// reasoning item 与 message 并存：只取 message 里的 output_text
	response := []byte(`{"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"hmm"}]},{"type":"message","content":[{"type":"output_text","text":"It is "},{"type":"refusal","refusal":null},{"type":"output_text","text":"red."}]}]}`)

	got, err := extractVisionText(consts.FormatOpenAIResponses, response)
	if err != nil {
		t.Fatalf("extractVisionText failed: %v", err)
	}
	if got != "It is\nred." {
		t.Fatalf("vision text mismatch: got %q", got)
	}
}

func TestExtractVisionText_OpenAIRes_NoMessage(t *testing.T) {
	response := []byte(`{"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"hmm"}]}]}`)

	if _, err := extractVisionText(consts.FormatOpenAIResponses, response); !errors.Is(err, errVisionTextNotFound) {
		t.Fatalf("expected errVisionTextNotFound, got %v", err)
	}
}

func TestVisionAnswerMatches(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "english lowercase", text: "The image is red.", want: true},
		{name: "english uppercase", text: "RED", want: true},
		{name: "english mixed case", text: "The color is Red", want: true},
		{name: "chinese", text: "这张图片是红色的。", want: true},
		{name: "traditional chinese", text: "這張圖片是紅色的。", want: true},
		{name: "bilingual", text: "The image appears red（红色）", want: true},
		{name: "refusal", text: "I cannot see any image.", want: false},
		{name: "chinese refusal", text: "我无法查看图片内容。", want: false},
		// red 必须词边界匹配：以下都是含 "red" 子串但不该命中的假阳性
		{name: "rendered substring", text: "the rendered content is not visible", want: false},
		{name: "stored substring", text: "The image is stored as PNG", want: false},
		{name: "colored substring", text: "It is a fully colored square", want: false},
		{name: "bordered substring", text: "a bordered square", want: false},
		{name: "reddish not word-boundary", text: "the square is reddish", want: false},
		{name: "empty", text: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := visionAnswerMatches(tc.text); got != tc.want {
				t.Fatalf("visionAnswerMatches(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestBuildVisionTestBody(t *testing.T) {
	// 三个已注册 type：测试体非空且字面量本身是合法 JSON（BuildReq 只注入 model）。
	// 这里补上 thinking 测试没做的 json.Valid——vision 体是三处拼接的复合字面量，
	// 引号/逗号手滑的代价是三个 provider 全量 400，必须在单测层拦住。
	for _, typ := range []string{providers.TypeOpenAI, providers.TypeOpenAIRes, providers.TypeAnthropic} {
		body, err := buildVisionTestBody(typ)
		if err != nil {
			t.Fatalf("buildVisionTestBody(%q) failed: %v", typ, err)
		}
		if len(body) == 0 {
			t.Fatalf("buildVisionTestBody(%q) returned empty body", typ)
		}
		if !json.Valid(body) {
			t.Fatalf("buildVisionTestBody(%q) produced invalid JSON", typ)
		}
	}

	if _, err := buildVisionTestBody("unknown-type"); !errors.Is(err, errInvalidProviderType) {
		t.Fatalf("expected errInvalidProviderType, got %v", err)
	}
}
