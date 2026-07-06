package transform

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestParseOpenAIResponse_MultipleChoicesAndUsageDetails(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"created":123,
		"model":"gpt-4",
		"choices":[
			{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"},
			{"index":1,"message":{"role":"assistant","content":"yo"},"finish_reason":"stop"}
		],
		"usage":{
			"prompt_tokens":1,
			"completion_tokens":2,
			"total_tokens":3,
			"prompt_tokens_details":{"cached_tokens":1,"audio_tokens":2},
			"completion_tokens_details":{"reasoning_tokens":5}
		}
	}`)

	unified, err := parseOpenAIResponse(body)
	if err != nil {
		t.Fatalf("parseOpenAIResponse: %v", err)
	}
	if unified == nil {
		t.Fatal("expected unified response, got nil")
	}
	if len(unified.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(unified.Choices))
	}
	if unified.Choices[0].Message == nil || unified.Choices[0].Message.GetContentAsString() != "hi" {
		t.Fatalf("unexpected choice[0] message: %#v", unified.Choices[0].Message)
	}
	if unified.Choices[1].Message == nil || unified.Choices[1].Message.GetContentAsString() != "yo" {
		t.Fatalf("unexpected choice[1] message: %#v", unified.Choices[1].Message)
	}
	if unified.Usage == nil {
		t.Fatal("expected usage to be set")
	}
	if unified.Usage.PromptTokensDetails.CachedTokens != 1 || unified.Usage.PromptTokensDetails.AudioTokens != 2 {
		t.Fatalf("unexpected prompt tokens details: %#v", unified.Usage.PromptTokensDetails)
	}
	if unified.Usage.CompletionTokensDetails.ReasoningTokens != 5 {
		t.Fatalf("unexpected completion tokens details: %#v", unified.Usage.CompletionTokensDetails)
	}
}

func TestOpenAIResponseCodec_ErrorEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":{"message":"nope","type":"invalid_request_error","code":"bad","param":"x","request_id":"req_1"}}`)
	unified, err := parseOpenAIResponse(body)
	if err != nil {
		t.Fatalf("parseOpenAIResponse: %v", err)
	}
	if unified.Error == nil {
		t.Fatal("expected unified.Error to be set")
	}
	if unified.Error.Detail.RequestID != "req_1" {
		t.Fatalf("expected request_id=req_1, got %q", unified.Error.Detail.RequestID)
	}

	out, err := formatOpenAIResponse(&models.UnifiedResponse{
		Error: &models.ResponseError{
			Detail: unified.Error.Detail,
		},
	})
	if err != nil {
		t.Fatalf("formatOpenAIResponse: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal formatted response: %v", err)
	}
	if _, ok := decoded["error"]; !ok {
		t.Fatalf("expected error envelope, got %s", string(out))
	}
	if _, ok := decoded["choices"]; ok {
		t.Fatalf("unexpected choices in error response: %s", string(out))
	}
}
