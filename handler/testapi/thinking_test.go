package testapi

import (
	"errors"
	"testing"

	"github.com/qkf688/llmux/consts"
)

func TestExtractThinking_OpenAI_ReasoningContent(t *testing.T) {
	// DeepSeek 系：推理内容在 reasoning_content
	reasoning := "2^10 = 1024，1024 - 8 = 1016。"
	response := []byte(`{"choices":[{"message":{"content":"1016","reasoning_content":` + jsonString(reasoning) + `}}]}`)

	got, err := extractThinking(consts.FormatOpenAIChat, response)
	if err != nil {
		t.Fatalf("extractThinking failed: %v", err)
	}
	if got != reasoning {
		t.Fatalf("thinking mismatch: got %q want %q", got, reasoning)
	}
}

func TestExtractThinking_OpenAI_Reasoning(t *testing.T) {
	// OpenAI o 系：推理内容在 reasoning
	reasoning := "Computing 2^10 - 8 = 1016."
	response := []byte(`{"choices":[{"message":{"content":"1016","reasoning":` + jsonString(reasoning) + `}}]}`)

	got, err := extractThinking(consts.FormatOpenAIChat, response)
	if err != nil {
		t.Fatalf("extractThinking failed: %v", err)
	}
	if got != reasoning {
		t.Fatalf("thinking mismatch: got %q want %q", got, reasoning)
	}
}

func TestExtractThinking_OpenAI_NotFound(t *testing.T) {
	// 上游忽略 reasoning_effort：响应只有 content，无任何推理字段
	response := []byte(`{"choices":[{"message":{"content":"1016"}}]}`)

	if _, err := extractThinking(consts.FormatOpenAIChat, response); !errors.Is(err, errThinkingNotFound) {
		t.Fatalf("expected errThinkingNotFound, got %v", err)
	}
}

func TestExtractThinking_OpenAI_ObjectShapedReasoningRejected(t *testing.T) {
	// 个别网关把 reasoning 回成对象（reasoning_details 类结构）：JSON 原文不是推理内容，按未返回处理
	response := []byte(`{"choices":[{"message":{"content":"1016","reasoning":{"effort":"medium"},"reasoning_content":{}}}]}`)

	if _, err := extractThinking(consts.FormatOpenAIChat, response); !errors.Is(err, errThinkingNotFound) {
		t.Fatalf("expected errThinkingNotFound for object-shaped reasoning, got %v", err)
	}
}

func TestExtractThinking_OpenAIRes(t *testing.T) {
	summary := "2^10 = 1024, minus 8 = 1016."
	response := []byte(`{"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":` + jsonString(summary) + `}]},{"type":"message","content":[{"type":"output_text","text":"1016"}]}]}`)

	got, err := extractThinking(consts.FormatOpenAIResponses, response)
	if err != nil {
		t.Fatalf("extractThinking failed: %v", err)
	}
	if got != summary {
		t.Fatalf("thinking mismatch: got %q want %q", got, summary)
	}
}

func TestExtractThinking_OpenAIRes_EncryptedOnly(t *testing.T) {
	// 纯推理模型只回 encrypted_content：reasoning item 存在即判通过，thinking_text 允许为空
	response := []byte(`{"output":[{"type":"reasoning","encrypted_content":"opaque"}]}`)

	got, err := extractThinking(consts.FormatOpenAIResponses, response)
	if err != nil {
		t.Fatalf("extractThinking failed: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty thinking text, got %q", got)
	}
}

func TestExtractThinking_OpenAIRes_NotFound(t *testing.T) {
	response := []byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"1016"}]}]}`)

	if _, err := extractThinking(consts.FormatOpenAIResponses, response); !errors.Is(err, errThinkingNotFound) {
		t.Fatalf("expected errThinkingNotFound, got %v", err)
	}
}

func TestExtractThinking_Anthropic(t *testing.T) {
	reasoning := "2^10 = 1024，1024 - 8 = 1016。"
	response := []byte(`{"content":[{"type":"thinking","thinking":` + jsonString(reasoning) + `},{"type":"text","text":"1016"}]}`)

	got, err := extractThinking(consts.FormatAnthropic, response)
	if err != nil {
		t.Fatalf("extractThinking failed: %v", err)
	}
	if got != reasoning {
		t.Fatalf("thinking mismatch: got %q want %q", got, reasoning)
	}
}

func TestExtractThinking_Anthropic_NotFound(t *testing.T) {
	// 中转平台省略 thinking block：content 只有 text
	response := []byte(`{"content":[{"type":"text","text":"1016"}]}`)

	if _, err := extractThinking(consts.FormatAnthropic, response); !errors.Is(err, errThinkingNotFound) {
		t.Fatalf("expected errThinkingNotFound, got %v", err)
	}
}

func TestExtractThinking_Anthropic_ObjectShapedThinkingRejected(t *testing.T) {
	// thinking 字段回对象（异常信封）：JSON 原文不是推理内容，按未返回处理
	response := []byte(`{"content":[{"type":"thinking","thinking":{"signature":"x"}}]}`)

	if _, err := extractThinking(consts.FormatAnthropic, response); !errors.Is(err, errThinkingNotFound) {
		t.Fatalf("expected errThinkingNotFound for object-shaped thinking, got %v", err)
	}
}

func TestBuildThinkingTestBody(t *testing.T) {
	// 三个已注册 type：测试体非空且为合法 JSON（BuildReq 只注入 model，先保证字面量本身合法）
	for _, typ := range []string{"openai", "openai-res", "anthropic"} {
		body, err := buildThinkingTestBody(typ)
		if err != nil {
			t.Fatalf("buildThinkingTestBody(%q) failed: %v", typ, err)
		}
		if len(body) == 0 {
			t.Fatalf("buildThinkingTestBody(%q) returned empty body", typ)
		}
	}

	if _, err := buildThinkingTestBody("unknown-type"); !errors.Is(err, errInvalidProviderType) {
		t.Fatalf("expected errInvalidProviderType, got %v", err)
	}
}
