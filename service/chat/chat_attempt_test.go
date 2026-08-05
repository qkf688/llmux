package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/qkf688/llmux/consts"
)

func TestApplyProviderSelectionResult_ReduceWeight(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{ReduceWeight: true})

	if weightItems[1] != 6 {
		t.Fatalf("expected weight reduced to 6, got %d", weightItems[1])
	}
	if priorityItems[1] != 7 {
		t.Fatalf("expected priority unchanged, got %d", priorityItems[1])
	}
}

func TestApplyProviderSelectionResult_RemoveWeightAndPriority(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{
		RemoveWeight:   true,
		RemovePriority: true,
	})

	if _, ok := weightItems[1]; ok {
		t.Fatal("expected weight entry removed")
	}
	if _, ok := priorityItems[1]; ok {
		t.Fatal("expected priority entry removed")
	}
}

func TestApplyProviderSelectionResult_RemoveOnlyWeight(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{RemoveWeight: true})

	if _, ok := weightItems[1]; ok {
		t.Fatal("expected weight entry removed")
	}
	if _, ok := priorityItems[1]; !ok {
		t.Fatal("expected priority entry kept")
	}
}

func TestBuildRequestBodyForProvider_OpenAI_MissingToolCallFunctionName_ReturnsHTTP400(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{
		"model":"m",
		"messages":[
			{
				"role":"assistant",
				"tool_calls":[
					{"type":"function","function":{"arguments":"{}"}}
				]
			}
		]
	}`)

	_, skip, err := buildRequestBodyForProvider(ctx, consts.StyleOpenAI, consts.StyleOpenAI, raw, nil, true)
	if skip {
		t.Fatalf("skipProvider = true, want false")
	}
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) {
		t.Fatalf("expected status-codable error, got %T: %v", err, err)
	}
	if got := statusCoder.StatusCode(); got != http.StatusBadRequest {
		t.Fatalf("StatusCode() = %d, want %d", got, http.StatusBadRequest)
	}
}

func TestBuildRequestBodyForProvider_OpenAI_ValidToolCall_Passes(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{
		"model":"m",
		"messages":[
			{
				"role":"assistant",
				"tool_calls":[
					{"type":"function","function":{"name":"container_shell","arguments":"{}"}}
				]
			}
		]
	}`)

	got, skip, err := buildRequestBodyForProvider(ctx, consts.StyleOpenAI, consts.StyleOpenAI, raw, nil, true)
	if skip {
		t.Fatalf("skipProvider = true, want false")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) == "" {
		t.Fatalf("expected non-empty body")
	}
}

func TestStripThinkingFields_Unsupported_RemovesFields(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":20000},"reasoning_effort":"high","messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["thinking"]; ok {
		t.Fatal("expected thinking field removed")
	}
	if _, ok := obj["reasoning_effort"]; ok {
		t.Fatal("expected reasoning_effort field removed")
	}
	if _, ok := obj["messages"]; !ok {
		t.Fatal("expected messages field kept")
	}
}

func TestStripThinkingFields_Supported_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":20000},"messages":[]}`)
	got := stripThinkingFields(body, true)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged when supportsThinking=true, got %s", got)
	}
}

func TestStripThinkingFields_NoThinkingFields_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","messages":[]}`)
	got := stripThinkingFields(body, false)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged when no thinking fields, got %s", got)
	}
}

func TestStripThinkingFields_InvalidJSON_Unchanged(t *testing.T) {
	body := []byte(`not-json`)
	got := stripThinkingFields(body, false)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged for invalid JSON, got %s", got)
	}
}

func TestStripThinkingFields_Reasoning_Removed(t *testing.T) {
	body := []byte(`{"model":"m","reasoning":{"effort":"high"},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["reasoning"]; ok {
		t.Fatal("expected reasoning field removed")
	}
}

// TestStripThinkingFields_PartialFields 真实场景：请求只带部分 thinking 键
// （Anthropic 只带 thinking、OpenAI 只带 reasoning_effort），缺失键应原样跳过。
func TestStripThinkingFields_PartialFields(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":20000},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["thinking"]; ok {
		t.Fatal("expected thinking field removed")
	}
	if _, ok := obj["reasoning_effort"]; ok {
		t.Fatal("unexpected reasoning_effort present")
	}
	if _, ok := obj["messages"]; !ok {
		t.Fatal("expected messages field kept")
	}
}

// TestStripThinkingFields_OutputConfigEffort_RemovedAndCleaned：
// Anthropic output_config.effort 是 adaptive thinking 字段，不支持 thinking 时必须删除，
// 删完后若 output_config 变空对象再删整个 output_config（避免残留空对象）。
func TestStripThinkingFields_OutputConfigEffort_RemovedAndCleaned(t *testing.T) {
	body := []byte(`{"model":"m","output_config":{"effort":"high"},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["output_config"]; ok {
		t.Fatal("expected output_config removed (was {effort:high}, after strip effort only key left → empty object → removed)")
	}
}

// TestStripThinkingFields_OutputConfigWithOtherFields_KeepsShell：
// output_config 含 effort 之外的字段时，删 effort 后 output_config 不空，应保留。
func TestStripThinkingFields_OutputConfigWithOtherFields_KeepsShell(t *testing.T) {
	body := []byte(`{"model":"m","output_config":{"effort":"high","format":"json"},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	oc, ok := obj["output_config"].(map[string]any)
	if !ok {
		t.Fatal("expected output_config kept (has format field left)")
	}
	if _, ok := oc["effort"]; ok {
		t.Fatal("expected output_config.effort removed")
	}
	if oc["format"] != "json" {
		t.Fatalf("expected output_config.format=json kept, got %v", oc["format"])
	}
}

func TestClampMaxTokens_OverLimit_Clamps(t *testing.T) {
	limit := 8192
	body := []byte(`{"model":"m","max_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 8192 {
		t.Fatalf("expected max_tokens=8192, got %v", obj["max_tokens"])
	}
}

func TestClampMaxTokens_UnderLimit_Unchanged(t *testing.T) {
	limit := 8192
	body := []byte(`{"model":"m","max_tokens":100,"messages":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestClampMaxTokens_NilLimit_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","max_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestClampMaxTokens_ZeroLimit_Unchanged(t *testing.T) {
	zero := 0
	body := []byte(`{"model":"m","max_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, &zero)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestClampMaxTokens_MaxCompletionTokens_Clamped(t *testing.T) {
	limit := 4096
	body := []byte(`{"model":"m","max_completion_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, _ := obj["max_completion_tokens"].(float64); int(v) != 4096 {
		t.Fatalf("expected max_completion_tokens=4096, got %v", obj["max_completion_tokens"])
	}
}
