package responses

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestParseResponse_ErrorEnvelope(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":{"message":"nope","type":"invalid_request_error","code":"bad","param":"x","request_id":"req_1"}}`)
	unified, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if unified.Error == nil {
		t.Fatal("expected unified.Error to be set")
	}
	if unified.Error.Detail.RequestID != "req_1" {
		t.Fatalf("expected request_id=req_1, got %q", unified.Error.Detail.RequestID)
	}
}

func TestParseResponse_FunctionCallOutput_IsPreservedAsText(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"object":"response",
		"id":"resp_1",
		"model":"m",
		"created_at":1,
		"status":"completed",
		"output":[
			{"type":"function_call_output","call_id":"call_1","output":"done"}
		]
	}`)
	unified, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(unified.Choices) != 1 || unified.Choices[0].Message == nil {
		t.Fatalf("unexpected choices: %#v", unified.Choices)
	}
	if got := unified.Choices[0].Message.GetContentAsString(); got != "done" {
		t.Fatalf("expected tool output text to be preserved, got %q", got)
	}
}

func TestFormatResponse_ErrorEnvelope(t *testing.T) {
	t.Parallel()

	unified := &models.UnifiedResponse{
		Error: &models.ResponseError{
			Detail: models.ErrorDetail{
				Message:   "nope",
				Type:      "invalid_request_error",
				Code:      "bad",
				Param:     "x",
				RequestID: "req_1",
			},
		},
	}
	out, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal formatted response: %v", err)
	}
	if _, ok := decoded["error"]; !ok {
		t.Fatalf("expected error envelope, got %s", string(out))
	}
	if _, ok := decoded["output"]; ok {
		t.Fatalf("unexpected output in error response: %s", string(out))
	}
}

func TestFormatResponse_EncodesImageURLParts(t *testing.T) {
	t.Parallel()

	txt := "hi"
	unified := &models.UnifiedResponse{
		ID:      "resp_1",
		Object:  "chat.completion",
		Created: 1,
		Model:   "m",
		Choices: []models.UnifiedChoice{{
			Index: 0,
			Message: &models.UnifiedMessage{
				Role: "assistant",
				Content: []models.UnifiedMessageContentPart{
					{Type: "text", Text: &txt},
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAA"}},
				},
			},
			FinishReason: "stop",
		}},
	}

	out, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse: %v", err)
	}

	var decoded ResponsesResponse
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal formatted response: %v", err)
	}
	if len(decoded.Output) == 0 {
		t.Fatalf("expected output items, got %#v", decoded)
	}

	foundImage := false
	for _, item := range decoded.Output {
		if item.Type != "message" {
			continue
		}
		content, ok := item.Content.([]interface{})
		if !ok {
			continue
		}
		for _, raw := range content {
			part, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if part["type"] == "image_url" && part["image_url"] == "data:image/png;base64,AAA" {
				foundImage = true
			}
		}
	}
	if !foundImage {
		t.Fatalf("expected image_url content part, got %s", string(out))
	}
}
