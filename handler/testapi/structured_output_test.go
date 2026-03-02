package testapi

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/consts"
)

func TestExtractStructuredOutputJSON_OpenAI(t *testing.T) {
	payload := `{"language":"zh","version":1,"features":["structured_output"],"score":1,"passed":true}`
	response := []byte(`{"choices":[{"message":{"content":` + jsonString(payload) + `}}]}`)

	gotPayload, gotRaw, err := extractStructuredOutputJSON(consts.StyleOpenAI, response)
	if err != nil {
		t.Fatalf("extractStructuredOutputJSON failed: %v", err)
	}
	if gotRaw != payload {
		t.Fatalf("raw mismatch: got %q want %q", gotRaw, payload)
	}
	if string(gotPayload) != payload {
		t.Fatalf("payload mismatch: got %s want %s", string(gotPayload), payload)
	}
}

func TestExtractStructuredOutputJSON_OpenAIRes(t *testing.T) {
	payload := `{"language":"zh","version":1,"features":["structured_output"],"score":1,"passed":true}`
	response := []byte(`{"output":[{"type":"output_text","text":` + jsonString(payload) + `}]}`)

	gotPayload, gotRaw, err := extractStructuredOutputJSON(consts.StyleOpenAIRes, response)
	if err != nil {
		t.Fatalf("extractStructuredOutputJSON failed: %v", err)
	}
	if gotRaw != payload {
		t.Fatalf("raw mismatch: got %q want %q", gotRaw, payload)
	}
	if string(gotPayload) != payload {
		t.Fatalf("payload mismatch: got %s want %s", string(gotPayload), payload)
	}
}

func TestExtractStructuredOutputJSON_Anthropic(t *testing.T) {
	payloadObj := `{"language":"zh","version":1,"features":["structured_output"],"score":1,"passed":true}`
	response := []byte(`{"content":[{"type":"tool_use","id":"1","name":"structured_output","input":` + payloadObj + `}]}`)

	gotPayload, gotRaw, err := extractStructuredOutputJSON(consts.StyleAnthropic, response)
	if err != nil {
		t.Fatalf("extractStructuredOutputJSON failed: %v", err)
	}
	if gotRaw != payloadObj {
		t.Fatalf("raw mismatch: got %q want %q", gotRaw, payloadObj)
	}
	if string(gotPayload) != payloadObj {
		t.Fatalf("payload mismatch: got %s want %s", string(gotPayload), payloadObj)
	}
}

func TestValidateStructuredOutputPayload(t *testing.T) {
	valid := json.RawMessage(`{"language":"zh","version":1,"features":["structured_output"],"score":1,"passed":true}`)
	if _, err := validateStructuredOutputPayload(valid); err != nil {
		t.Fatalf("expected valid payload, got error: %v", err)
	}

	invalidExtra := json.RawMessage(`{"language":"zh","version":1,"features":["structured_output"],"score":1,"passed":true,"extra":1}`)
	if _, err := validateStructuredOutputPayload(invalidExtra); err == nil {
		t.Fatal("expected error for extra key, got nil")
	}

	invalidMissing := json.RawMessage(`{"language":"zh","version":1,"features":["structured_output"],"score":1}`)
	if _, err := validateStructuredOutputPayload(invalidMissing); err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
