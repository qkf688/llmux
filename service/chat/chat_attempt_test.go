package chat

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/atopos31/llmio/consts"
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

	_, skip, err := buildRequestBodyForProvider(ctx, consts.StyleOpenAI, consts.StyleOpenAI, raw)
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

	got, skip, err := buildRequestBodyForProvider(ctx, consts.StyleOpenAI, consts.StyleOpenAI, raw)
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
