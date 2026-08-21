package anthropic

import (
	"testing"

	"github.com/tidwall/gjson"

	"github.com/qkf688/llmux/models"
)

// TestFormatResponse_ErrorEnvelope 冻结「纯错误响应」的出站形状。
//
// 此前本函数无 unified.Error 分支：上游返回 200 + error body 时，Anthropic 客户端
// 拿到的是 {"id":"","type":"message","role":"assistant","model":"","content":[]}
// ——形式合法但语义空洞，错误的 type/message/request_id 整体丢失。
// 断言打在响应体 JSON 上（而非 Go 结构体），因为要测的正是键名与键存在性。
func TestFormatResponse_ErrorEnvelope(t *testing.T) {
	out, err := FormatResponse(&models.UnifiedResponse{
		ID:    "resp-error",
		Model: "coverage-model",
		Error: &models.ResponseError{
			Detail: models.ErrorDetail{
				Code:      "rate_limited",
				Message:   "too many requests",
				Type:      "rate_limit_error",
				Param:     "max_tokens",
				RequestID: "req-error",
			},
		},
	})
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}

	body := gjson.ParseBytes(out)

	if got := body.Get("type").String(); got != "error" {
		t.Errorf("top-level type = %q, want \"error\"", got)
	}
	if got := body.Get("error.type").String(); got != "rate_limit_error" {
		t.Errorf("error.type = %q, want \"rate_limit_error\"", got)
	}
	if got := body.Get("error.message").String(); got != "too many requests" {
		t.Errorf("error.message = %q, want \"too many requests\"", got)
	}
	if got := body.Get("request_id").String(); got != "req-error" {
		t.Errorf("request_id = %q, want \"req-error\"", got)
	}

	// message 形状的键一个都不能出现——否则客户端会把错误当成一条空回复渲染。
	for _, key := range []string{"role", "content", "model", "stop_reason", "usage"} {
		if body.Get(key).Exists() {
			t.Errorf("key %q must not exist in error envelope, got %q", key, body.Get(key).Raw)
		}
	}

	// code / param 不是 Anthropic 契约字段，刻意不写（同 usage 段的按协议裁剪口径）。
	for _, key := range []string{"error.code", "error.param", "error.request_id"} {
		if body.Get(key).Exists() {
			t.Errorf("key %q is not an Anthropic error field, got %q", key, body.Get(key).Raw)
		}
	}
}

// TestFormatResponse_ErrorEnvelopeOmitsEmptyRequestID 区分「上游给了 request_id」与
// 「上游没给」：空值必须让键整个缺席，而不是写出 "request_id":""——后者会让客户端
// 以为拿到了一个可用于联系支持的 ID。
func TestFormatResponse_ErrorEnvelopeOmitsEmptyRequestID(t *testing.T) {
	out, err := FormatResponse(&models.UnifiedResponse{
		Error: &models.ResponseError{
			Detail: models.ErrorDetail{
				Message: "upstream exploded",
				Type:    "rate_limit_exceeded", // 非 Anthropic 枚举 → 兜底 api_error
			},
		},
	})
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}

	body := gjson.ParseBytes(out)

	if body.Get("request_id").Exists() {
		t.Errorf("request_id must be absent when upstream gave none, got %q", body.Get("request_id").Raw)
	}
	if got := body.Get("error.type").String(); got != "api_error" {
		t.Errorf("error.type = %q, want \"api_error\" (unknown type falls back)", got)
	}
}

// TestFormatResponse_ErrorWithChoicesKeepsMessageShape 冻结守卫口径：Error 与 Choices
// 并存时走正常 message 分支，与 openai / openai-res 出站的
// `Error != nil && len(Choices) == 0` 判据保持一致。
// （并存时错误被丢弃是三协议共有的已知缺陷，本次刻意不改判据。）
func TestFormatResponse_ErrorWithChoicesKeepsMessageShape(t *testing.T) {
	content := "hello"
	out, err := FormatResponse(&models.UnifiedResponse{
		ID:    "resp-both",
		Model: "coverage-model",
		Choices: []models.UnifiedChoice{{
			Message:      &models.UnifiedMessage{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
		Error: &models.ResponseError{
			Detail: models.ErrorDetail{Message: "ignored", Type: "api_error"},
		},
	})
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}

	body := gjson.ParseBytes(out)

	if got := body.Get("type").String(); got != "message" {
		t.Errorf("top-level type = %q, want \"message\" when choices present", got)
	}
	if got := body.Get("content.0.text").String(); got != content {
		t.Errorf("content.0.text = %q, want %q", got, content)
	}
	if body.Get("error").Exists() {
		t.Errorf("error key must not appear in message shape, got %q", body.Get("error").Raw)
	}
}
