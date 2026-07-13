package handler

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/handler/logs"
	"github.com/atopos31/llmio/models"
)

func TestClearFilteredLogs_RequiresFilters(t *testing.T) {
	initHandlerTestDB(t)

	c, w := newHandlerTestContext("DELETE", "/logs/clear-filtered")
	logs.ClearFilteredLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 400 {
		t.Fatalf("payload code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}

func TestClearFilteredLogs_StatusSuccess(t *testing.T) {
	initHandlerTestDB(t)

	chatLogs := []models.ChatLog{
		{Name: "m1", ProviderName: "p1", ProviderModel: "pm1", Status: "success", Style: "openai"},
		{Name: "m1", ProviderName: "p1", ProviderModel: "pm1", Status: "success", Style: "openai"},
		{Name: "m1", ProviderName: "p1", ProviderModel: "pm1", Status: "error", Style: "openai", Error: "boom"},
	}
	for i := range chatLogs {
		if err := models.DB.Create(&chatLogs[i]).Error; err != nil {
			t.Fatalf("create log %d: %v", i, err)
		}
	}

	chatIO := models.ChatIO{
		LogId: chatLogs[0].ID,
		Input: "hi",
		OutputUnion: models.OutputUnion{
			OfString: "out",
		},
	}
	if err := models.DB.Create(&chatIO).Error; err != nil {
		t.Fatalf("create chat io: %v", err)
	}

	c, w := newHandlerTestContext("DELETE", "/logs/clear-filtered?status=success")
	logs.ClearFilteredLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[struct {
		Deleted int64 `json:"deleted"`
	}]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if payload.Data.Deleted != 2 {
		t.Fatalf("deleted = %d, want 2", payload.Data.Deleted)
	}

	var remainingSuccess int64
	if err := models.DB.Model(&models.ChatLog{}).Where("status = ?", "success").Count(&remainingSuccess).Error; err != nil {
		t.Fatalf("count remaining success: %v", err)
	}
	if remainingSuccess != 0 {
		t.Fatalf("remaining success = %d, want 0", remainingSuccess)
	}

	var remainingError int64
	if err := models.DB.Model(&models.ChatLog{}).Where("status = ?", "error").Count(&remainingError).Error; err != nil {
		t.Fatalf("count remaining error: %v", err)
	}
	if remainingError != 1 {
		t.Fatalf("remaining error = %d, want 1", remainingError)
	}

	var remainingChatIO int64
	if err := models.DB.Model(&models.ChatIO{}).Where("log_id = ?", chatLogs[0].ID).Count(&remainingChatIO).Error; err != nil {
		t.Fatalf("count remaining chat io: %v", err)
	}
	if remainingChatIO != 0 {
		t.Fatalf("remaining chat io = %d, want 0", remainingChatIO)
	}
}
