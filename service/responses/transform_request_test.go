package responses

import (
	"encoding/json"
	"testing"
)

func TestTransformRequest_RoundTrip_StringInputPreserved(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":"hi"}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.TransformOptions.ArrayInputs == nil || *unified.TransformOptions.ArrayInputs {
		t.Fatalf("expected ArrayInputs=false, got: %#v", unified.TransformOptions.ArrayInputs)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var req ResponsesRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("failed to unmarshal responses request: %v", err)
	}
	if req.Input.Text == nil || *req.Input.Text != "hi" {
		t.Fatalf("expected input to be string 'hi', got: %#v", req.Input)
	}
	if req.Input.Items != nil {
		t.Fatalf("expected input items to be nil, got: %#v", req.Input.Items)
	}
}

func TestTransformRequest_RoundTrip_ArrayInputPreserved(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.TransformOptions.ArrayInputs == nil || !*unified.TransformOptions.ArrayInputs {
		t.Fatalf("expected ArrayInputs=true, got: %#v", unified.TransformOptions.ArrayInputs)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	input := m["input"]
	if len(input) == 0 || input[0] != '[' {
		t.Fatalf("expected output input to be array, got: %s", string(input))
	}
}

func TestTransformRequest_ReasoningMaxTokensMappedToBudget(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":"hi","reasoning":{"effort":"low","max_tokens":123}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 123 {
		t.Fatalf("expected unified.ReasoningBudget=123, got: %#v", unified.ReasoningBudget)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var req ResponsesRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("failed to unmarshal responses request: %v", err)
	}
	if req.Reasoning == nil || req.Reasoning.MaxTokens == nil || *req.Reasoning.MaxTokens != 123 {
		t.Fatalf("expected responses.reasoning.max_tokens=123, got: %#v", req.Reasoning)
	}
	if req.Reasoning.Effort == nil || *req.Reasoning.Effort != "low" {
		t.Fatalf("expected responses.reasoning.effort=low, got: %#v", req.Reasoning)
	}
}

// TestTransformRequest_CacheSecurityFields_RoundTrip 覆盖 service_tier /
// safety_identifier / prompt_cache_key 的入站解析与出站回写对称性。
// metadata 是客户端自由 KV 标签，不是 effort 的来源。官方 reasoning.effort 缺席时，
// metadata.reasoning_effort 只作为普通标签透传进 unified.Metadata，不反推 ReasoningEffort。
func TestTransformRequest_MetadataReasoningEffortNotUsedAsEffort(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":"hi","metadata":{"reasoning_effort":"high"}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.ReasoningEffort != nil {
		t.Fatalf("ReasoningEffort should be nil (metadata is not a control channel), got %q", *unified.ReasoningEffort)
	}
	if unified.Metadata["reasoning_effort"] != "high" {
		t.Fatalf("metadata tag should pass through, got %#v", unified.Metadata)
	}
}

func TestTransformRequest_CacheSecurityFields_RoundTrip(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":"hi","service_tier":"flex","safety_identifier":"user-hash-123","prompt_cache_key":"cache-abc"}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.ServiceTier == nil || *unified.ServiceTier != "flex" {
		t.Fatalf("service_tier not parsed: %#v", unified.ServiceTier)
	}
	if unified.SafetyIdentifier == nil || *unified.SafetyIdentifier != "user-hash-123" {
		t.Fatalf("safety_identifier not parsed: %#v", unified.SafetyIdentifier)
	}
	if unified.PromptCacheKey == nil || *unified.PromptCacheKey != "cache-abc" {
		t.Fatalf("prompt_cache_key not parsed: %#v", unified.PromptCacheKey)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}
	var req ResponsesRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("failed to unmarshal responses request: %v", err)
	}
	if req.ServiceTier == nil || *req.ServiceTier != "flex" {
		t.Fatalf("service_tier not emitted: %#v", req.ServiceTier)
	}
	if req.SafetyIdentifier == nil || *req.SafetyIdentifier != "user-hash-123" {
		t.Fatalf("safety_identifier not emitted: %#v", req.SafetyIdentifier)
	}
	if req.PromptCacheKey == nil || *req.PromptCacheKey != "cache-abc" {
		t.Fatalf("prompt_cache_key not emitted: %#v", req.PromptCacheKey)
	}
}

// 出站 effort 只 emit 到官方 reasoning.effort：metadata 属客户端命名空间
// （OpenAI 限 16 键），不得掺入 llmux 自己的控制字段。
func TestTransformFromUnified_EffortNotWrittenIntoMetadata(t *testing.T) {
	t.Run("无业务标签时不产出 metadata 键", func(t *testing.T) {
		unified, err := TransformRequest([]byte(`{"model":"gpt-4.1","input":"hi","reasoning":{"effort":"high"}}`), RequestTransformOptions{})
		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}
		out, err := TransformFromUnified(unified)
		if err != nil {
			t.Fatalf("TransformFromUnified failed: %v", err)
		}

		var body map[string]json.RawMessage
		if err := json.Unmarshal(out, &body); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if _, ok := body["metadata"]; ok {
			t.Fatalf("metadata key should be absent, got %s", out)
		}

		var req ResponsesRequest
		if err := json.Unmarshal(out, &req); err != nil {
			t.Fatalf("failed to unmarshal responses request: %v", err)
		}
		if req.Reasoning == nil || req.Reasoning.Effort == nil || *req.Reasoning.Effort != "high" {
			t.Fatalf("reasoning.effort not emitted: %#v", req.Reasoning)
		}
	})

	t.Run("有业务标签时标签保留且不掺入 effort", func(t *testing.T) {
		unified, err := TransformRequest([]byte(`{"model":"gpt-4.1","input":"hi","reasoning":{"effort":"high"},"metadata":{"trace_id":"abc"}}`), RequestTransformOptions{})
		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}
		out, err := TransformFromUnified(unified)
		if err != nil {
			t.Fatalf("TransformFromUnified failed: %v", err)
		}

		var req ResponsesRequest
		if err := json.Unmarshal(out, &req); err != nil {
			t.Fatalf("failed to unmarshal responses request: %v", err)
		}
		if req.Metadata["trace_id"] != "abc" {
			t.Fatalf("trace_id tag lost: %#v", req.Metadata)
		}
		if _, ok := req.Metadata["reasoning_effort"]; ok {
			t.Fatalf("metadata should not contain reasoning_effort: %#v", req.Metadata)
		}
	})
}
