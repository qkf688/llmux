package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
)

func TestMetricsHourliesToday_FillsMissingHours(t *testing.T) {
	initHandlerTestDB(t)

	now := time.Now()
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	if err := models.DB.Create(&models.StatsHourly{Date: date, Hour: 0, Reqs: 2, Tokens: 10}).Error; err != nil {
		t.Fatalf("create hour 0: %v", err)
	}
	if err := models.DB.Create(&models.StatsHourly{Date: date, Hour: 13, Reqs: 5, Tokens: 99}).Error; err != nil {
		t.Fatalf("create hour 13: %v", err)
	}
	if err := models.DB.Create(&models.StatsHourly{Date: date, Hour: 23, Reqs: 1, Tokens: 7}).Error; err != nil {
		t.Fatalf("create hour 23: %v", err)
	}

	c, w := newHandlerTestContext("GET", "/metrics/hourlies/today")
	MetricsHourliesToday(c)

	if w.Code != 200 {
		t.Fatalf("status code=%d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[[]HourlyMetricsRes]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code=%d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data) != 24 {
		t.Fatalf("len(data)=%d, want 24", len(payload.Data))
	}
	if payload.Data[0].Hour != 0 || payload.Data[0].Reqs != 2 || payload.Data[0].Tokens != 10 {
		t.Fatalf("hour0=%+v, want hour=0 reqs=2 tokens=10", payload.Data[0])
	}
	if payload.Data[1].Hour != 1 || payload.Data[1].Reqs != 0 || payload.Data[1].Tokens != 0 {
		t.Fatalf("hour1=%+v, want hour=1 reqs=0 tokens=0", payload.Data[1])
	}
	if payload.Data[13].Hour != 13 || payload.Data[13].Reqs != 5 || payload.Data[13].Tokens != 99 {
		t.Fatalf("hour13=%+v, want hour=13 reqs=5 tokens=99", payload.Data[13])
	}
	if payload.Data[23].Hour != 23 || payload.Data[23].Reqs != 1 || payload.Data[23].Tokens != 7 {
		t.Fatalf("hour23=%+v, want hour=23 reqs=1 tokens=7", payload.Data[23])
	}
}
