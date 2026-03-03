package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type apiEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Data    T      `json:"data"`
}

type modelSyncLogsResponse struct {
	Data       []models.ModelSyncLog `json:"data"`
	Pagination struct {
		Page       int   `json:"page"`
		PageSize   int   `json:"page_size"`
		Total      int64 `json:"total"`
		TotalPages int64 `json:"total_pages"`
	} `json:"pagination"`
}

func initHandlerTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	t.Cleanup(func() {
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

func newHandlerTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}

func TestGetModelSyncLogs_StatusFilter(t *testing.T) {
	initHandlerTestDB(t)

	provider := models.Provider{Name: "p1", Type: "openai"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	now := time.Now()
	logs := []models.ModelSyncLog{
		{
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			Status:       "success",
			SyncedAt:     now.Add(-3 * time.Hour),
		},
		{
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			Status:       "error",
			Error:        "status code: 401 response: unauthorized",
			SyncedAt:     now.Add(-2 * time.Hour),
		},
		{
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			Status:       "error",
			Error:        "status code: 429 response: rate limited",
			SyncedAt:     now.Add(-1 * time.Hour),
		},
	}
	for i := range logs {
		if err := models.DB.Create(&logs[i]).Error; err != nil {
			t.Fatalf("create sync log %d: %v", i, err)
		}
	}

	c, w := newHandlerTestContext("GET", "/model-sync/logs?status=error&page_size=100")
	GetModelSyncLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[modelSyncLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data.Data) != 2 {
		t.Fatalf("filtered logs length = %d, want 2", len(payload.Data.Data))
	}
	for _, log := range payload.Data.Data {
		if log.Status != "error" {
			t.Fatalf("unexpected log status: %q", log.Status)
		}
	}
}

func TestGetModelSyncLogs_InvalidStatus(t *testing.T) {
	initHandlerTestDB(t)

	c, w := newHandlerTestContext("GET", "/model-sync/logs?status=bad")
	GetModelSyncLogs(c)
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

func TestGetProviderModels_ModelEndpointDisabled(t *testing.T) {
	initHandlerTestDB(t)

	disabled := false
	provider := models.Provider{
		Name:          "p1",
		Type:          "openai",
		Config:        `{"api_key":"k","base_url":"https://example.com"}`,
		ModelEndpoint: &disabled,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	c, w := newHandlerTestContext("GET", "/providers/models/"+strconv.FormatUint(uint64(provider.ID), 10))
	c.Params = []gin.Param{{Key: "id", Value: strconv.FormatUint(uint64(provider.ID), 10)}}
	GetProviderModels(c)
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
