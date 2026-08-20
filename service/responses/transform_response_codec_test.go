package responses

import (
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/models"
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

// OpenAI Responses 的 output 数组内 message item id 必须唯一：官方 schema 把
// ResponseOutputMessage.id 定义为「the unique ID of the output message」，而客户端把
// response.output 回灌成下一轮 input 是该协议的常规续话方式，重复的 msg_ id 会被 API 判
// "Duplicate item found with id msg_..." 而 400 —— 危害出现在客户端的下一次请求，只看
// 单次响应体察觉不到。多 choice 时若每个 choice 的 message 都取 msg_+unified.ID 就会撞车。
func TestFormatResponse_MultiChoiceMessageItemIDsAreUnique(t *testing.T) {
	t.Parallel()

	first := "first"
	second := "second"
	unified := &models.UnifiedResponse{
		ID:      "resp_multi",
		Object:  "chat.completion",
		Created: 1,
		Model:   "m",
		Choices: []models.UnifiedChoice{
			{Index: 0, FinishReason: "stop", Message: &models.UnifiedMessage{Role: "assistant", Content: first}},
			{Index: 1, FinishReason: "stop", Message: &models.UnifiedMessage{Role: "assistant", Content: second}},
		},
	}

	out, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse: %v", err)
	}

	var decoded ResponsesResponse
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal formatted response: %v", err)
	}

	var messageIDs []string
	for _, item := range decoded.Output {
		if item.Type == "message" {
			messageIDs = append(messageIDs, item.ID)
		}
	}
	if len(messageIDs) != 2 {
		t.Fatalf("expected 2 message items, got %d (%v)", len(messageIDs), messageIDs)
	}
	// 首个保持裸 msg_<id>：单 choice 是绝对主流形态，给它加后缀等于改动几乎所有响应的 id。
	if messageIDs[0] != "msg_resp_multi" {
		t.Fatalf("first message id should stay bare msg_<id>, got %q", messageIDs[0])
	}
	// 全员去重而非两两比较：choice 数增加时这条断言自动仍然有效。
	seen := make(map[string]bool, len(messageIDs))
	for _, id := range messageIDs {
		if seen[id] {
			t.Fatalf("duplicate message item id %q in output array: %v", id, messageIDs)
		}
		seen[id] = true
	}
}
