package transform

import (
	"context"
	"encoding/json"
	"github.com/atopos31/llmio/models"
	"testing"
)

// 阶段 3: 测试多模态内容支持
func TestTransformOpenAIToUnified_MultimodalContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *models.UnifiedRequest)
	}{
		{
			name: "text_only",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if len(u.Messages) != 1 {
					t.Fatal("expected 1 message")
				}
				if str, ok := u.Messages[0].Content.(string); !ok || str != "Hello" {
					t.Error("content should be string")
				}
			},
		},
		{
			name: "text_and_image",
			input: `{
				"model": "gpt-4-vision",
				"messages": [{
					"role": "user",
					"content": [
						{"type": "text", "text": "What's in this image?"},
						{"type": "image_url", "image_url": {"url": "https://example.com/image.jpg", "detail": "high"}}
					]
				}]
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if len(u.Messages) != 1 {
					t.Fatal("expected 1 message")
				}
				parts, ok := u.Messages[0].Content.([]models.UnifiedMessageContentPart)
				if !ok {
					t.Fatal("content should be []models.UnifiedMessageContentPart")
				}
				if len(parts) != 2 {
					t.Fatalf("expected 2 parts, got %d", len(parts))
				}
				if parts[0].Type != "text" || parts[0].Text == nil || *parts[0].Text != "What's in this image?" {
					t.Error("first part should be text")
				}
				if parts[1].Type != "image_url" || parts[1].ImageURL == nil {
					t.Error("second part should be image_url")
				}
				if parts[1].ImageURL.URL != "https://example.com/image.jpg" {
					t.Error("image URL mismatch")
				}
				if parts[1].ImageURL.Detail == nil || *parts[1].ImageURL.Detail != "high" {
					t.Error("image detail mismatch")
				}
			},
		},
		{
			name: "audio_input",
			input: `{
				"model": "gpt-4-audio",
				"messages": [{
					"role": "user",
					"content": [
						{"type": "text", "text": "Transcribe this audio"},
						{"type": "input_audio", "input_audio": {"data": "base64data", "format": "wav"}}
					]
				}]
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if len(u.Messages) != 1 {
					t.Fatal("expected 1 message")
				}
				parts, ok := u.Messages[0].Content.([]models.UnifiedMessageContentPart)
				if !ok {
					t.Fatal("content should be []models.UnifiedMessageContentPart")
				}
				if len(parts) != 2 {
					t.Fatalf("expected 2 parts, got %d", len(parts))
				}
				if parts[1].Type != "input_audio" || parts[1].InputAudio == nil {
					t.Error("second part should be input_audio")
				}
				if parts[1].InputAudio.Data != "base64data" {
					t.Error("audio data mismatch")
				}
				if parts[1].InputAudio.Format != "wav" {
					t.Error("audio format mismatch")
				}
			},
		},
		{
			name: "modalities_and_audio",
			input: `{
				"model": "gpt-4-audio",
				"messages": [{"role": "user", "content": "Generate audio"}],
				"modalities": ["text", "audio"],
				"audio": {"voice": "alloy", "format": "wav"}
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if len(u.Modalities) != 2 {
					t.Fatalf("expected 2 modalities, got %d", len(u.Modalities))
				}
				if u.Modalities[0] != "text" || u.Modalities[1] != "audio" {
					t.Error("modalities mismatch")
				}
				if u.Audio == nil {
					t.Fatal("audio config is nil")
				}
				if u.Audio.Voice != "alloy" {
					t.Error("audio voice mismatch")
				}
				if u.Audio.Format != "wav" {
					t.Error("audio format mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformOpenAIToUnified(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, unified)
		})
	}
}

func TestTransformUnifiedToOpenAI_MultimodalContent(t *testing.T) {
	// 测试纯文本 (向后兼容)
	unified1 := &models.UnifiedRequest{
		Model:    "gpt-4",
		Messages: []models.UnifiedMessage{{Role: "user", Content: "Hello"}},
	}

	result1, err := TransformUnifiedToOpenAI(unified1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed1 map[string]interface{}
	if err := json.Unmarshal(result1, &parsed1); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages1 := parsed1["messages"].([]interface{})
	msg1 := messages1[0].(map[string]interface{})
	if msg1["content"] != "Hello" {
		t.Error("text content mismatch")
	}

	// 测试多模态内容
	text := "What's in this image?"
	detail := "high"
	unified2 := &models.UnifiedRequest{
		Model: "gpt-4-vision",
		Messages: []models.UnifiedMessage{{
			Role: "user",
			Content: []models.UnifiedMessageContentPart{
				{Type: "text", Text: &text},
				{Type: "image_url", ImageURL: &models.UnifiedImageURL{
					URL:    "https://example.com/image.jpg",
					Detail: &detail,
				}},
			},
		}},
	}

	result2, err := TransformUnifiedToOpenAI(unified2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed2 map[string]interface{}
	if err := json.Unmarshal(result2, &parsed2); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages2 := parsed2["messages"].([]interface{})
	msg2 := messages2[0].(map[string]interface{})
	contentArray := msg2["content"].([]interface{})
	if len(contentArray) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(contentArray))
	}

	part1 := contentArray[0].(map[string]interface{})
	if part1["type"] != "text" || part1["text"] != "What's in this image?" {
		t.Error("text part mismatch")
	}

	part2 := contentArray[1].(map[string]interface{})
	if part2["type"] != "image_url" {
		t.Error("image_url type mismatch")
	}

	// 测试 Modalities 和 Audio
	unified3 := &models.UnifiedRequest{
		Model:      "gpt-4-audio",
		Messages:   []models.UnifiedMessage{{Role: "user", Content: "Generate audio"}},
		Modalities: []string{"text", "audio"},
		Audio: &models.UnifiedAudio{
			Voice:  "alloy",
			Format: "wav",
		},
	}

	result3, err := TransformUnifiedToOpenAI(unified3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed3 map[string]interface{}
	if err := json.Unmarshal(result3, &parsed3); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	modalities := parsed3["modalities"].([]interface{})
	if len(modalities) != 2 {
		t.Error("modalities mismatch")
	}

	audio := parsed3["audio"].(map[string]interface{})
	if audio["voice"] != "alloy" || audio["format"] != "wav" {
		t.Error("audio config mismatch")
	}
}

func TestUnifiedMessage_Helpers(t *testing.T) {
	// 测试 GetContentAsString - 纯文本
	msg1 := models.UnifiedMessage{Content: "Hello"}
	if msg1.GetContentAsString() != "Hello" {
		t.Error("GetContentAsString failed for string")
	}

	// 测试 GetContentAsString - 多模态
	text1 := "Part 1"
	text2 := "Part 2"
	msg2 := models.UnifiedMessage{
		Content: []models.UnifiedMessageContentPart{
			{Type: "text", Text: &text1},
			{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "http://example.com/img.jpg"}},
			{Type: "text", Text: &text2},
		},
	}
	if msg2.GetContentAsString() != "Part 1Part 2" {
		t.Error("GetContentAsString failed for multimodal")
	}

	// 测试 GetContentParts - 纯文本
	parts1 := msg1.GetContentParts()
	if len(parts1) != 1 || parts1[0].Type != "text" || *parts1[0].Text != "Hello" {
		t.Error("GetContentParts failed for string")
	}

	// 测试 GetContentParts - 多模态
	parts2 := msg2.GetContentParts()
	if len(parts2) != 3 {
		t.Error("GetContentParts failed for multimodal")
	}

	// 测试 SetContentString
	msg3 := models.UnifiedMessage{}
	msg3.SetContentString("Test")
	if msg3.Content != "Test" {
		t.Error("SetContentString failed")
	}

	// 测试 SetContentParts
	msg4 := models.UnifiedMessage{}
	msg4.SetContentParts([]models.UnifiedMessageContentPart{
		{Type: "text", Text: &text1},
	})
	if parts, ok := msg4.Content.([]models.UnifiedMessageContentPart); !ok || len(parts) != 1 {
		t.Error("SetContentParts failed")
	}
}

func TestUnifiedMessageContent_JSON(t *testing.T) {
	// 测试纯文本序列化
	content1 := UnifiedMessageContent{Content: func() *string { s := "Hello"; return &s }()}
	data1, err := json.Marshal(content1)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data1) != `"Hello"` {
		t.Errorf("expected \"Hello\", got %s", string(data1))
	}

	// 测试多模态序列化
	text := "Test"
	content2 := UnifiedMessageContent{
		MultipleContent: []models.UnifiedMessageContentPart{
			{Type: "text", Text: &text},
		},
	}
	data2, err := json.Marshal(content2)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// 单个 text 部分应该优化为字符串
	if string(data2) != `"Test"` {
		t.Errorf("expected \"Test\", got %s", string(data2))
	}

	// 测试多个部分序列化
	content3 := UnifiedMessageContent{
		MultipleContent: []models.UnifiedMessageContentPart{
			{Type: "text", Text: &text},
			{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "http://example.com/img.jpg"}},
		},
	}
	data3, err := json.Marshal(content3)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed []interface{}
	if err := json.Unmarshal(data3, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(parsed) != 2 {
		t.Error("expected 2 parts in array")
	}

	// 测试纯文本反序列化
	var content4 UnifiedMessageContent
	if err := json.Unmarshal([]byte(`"Hello"`), &content4); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if content4.Content == nil || *content4.Content != "Hello" {
		t.Error("string unmarshal failed")
	}

	// 测试多模态反序列化
	var content5 UnifiedMessageContent
	if err := json.Unmarshal([]byte(`[{"type":"text","text":"Test"}]`), &content5); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(content5.MultipleContent) != 1 || content5.MultipleContent[0].Type != "text" {
		t.Error("array unmarshal failed")
	}
}
