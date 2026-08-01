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

	_, skip, err := buildRequestBodyForProvider(ctx, consts.StyleOpenAI, consts.StyleOpenAI, raw, nil)
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

	got, skip, err := buildRequestBodyForProvider(ctx, consts.StyleOpenAI, consts.StyleOpenAI, raw, nil)
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
