package handler

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestMetricsTotal_ReturnsStatsTotalOrZero(t *testing.T) {
	initHandlerTestDB(t)

	// No row -> zeros
	{
		c, w := newHandlerTestContext("GET", "/metrics/total")
		MetricsTotal(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var payload apiEnvelope[MetricsRes]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if payload.Data.Reqs != 0 || payload.Data.Tokens != 0 {
			t.Fatalf("metrics total = %+v, want reqs=0 tokens=0", payload.Data)
		}
	}

	// Row exists -> return it
	{
		if err := models.DB.Create(&models.StatsTotal{ID: 1, Reqs: 12, Tokens: 345}).Error; err != nil {
			t.Fatalf("create stats total: %v", err)
		}

		c, w := newHandlerTestContext("GET", "/metrics/total")
		MetricsTotal(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var payload apiEnvelope[MetricsRes]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if payload.Data.Reqs != 12 || payload.Data.Tokens != 345 {
			t.Fatalf("metrics total = %+v, want reqs=12 tokens=345", payload.Data)
		}
	}
}

