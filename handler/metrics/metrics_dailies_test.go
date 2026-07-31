package metrics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

func TestMetricsDailies_ReturnsRowsSinceStartDate(t *testing.T) {
	testsupport.InitTestDB(t)

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	older := now.AddDate(0, 0, -10).Format("2006-01-02")

	if err := models.DB.Create(&models.StatsDaily{Date: older, Reqs: 1, Tokens: 2}).Error; err != nil {
		t.Fatalf("create older daily: %v", err)
	}
	if err := models.DB.Create(&models.StatsDaily{Date: yesterday, Reqs: 3, Tokens: 4}).Error; err != nil {
		t.Fatalf("create yesterday daily: %v", err)
	}
	if err := models.DB.Create(&models.StatsDaily{Date: today, Reqs: 5, Tokens: 6}).Error; err != nil {
		t.Fatalf("create today daily: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/metrics/dailies/2")
	c.Params = gin.Params{{Key: "days", Value: "2"}}
	MetricsDailies(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[[]DailyMetricsRes]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}

	if len(payload.Data) != 2 {
		t.Fatalf("rows length = %d, want 2, rows=%+v", len(payload.Data), payload.Data)
	}
	if payload.Data[0].Date != yesterday || payload.Data[0].Reqs != 3 || payload.Data[0].Tokens != 4 {
		t.Fatalf("row0=%+v, want %s reqs=3 tokens=4", payload.Data[0], yesterday)
	}
	if payload.Data[1].Date != today || payload.Data[1].Reqs != 5 || payload.Data[1].Tokens != 6 {
		t.Fatalf("row1=%+v, want %s reqs=5 tokens=6", payload.Data[1], today)
	}
}

func TestMetricsDailies_InvalidDays(t *testing.T) {
	testsupport.InitTestDB(t)

	c, w := testsupport.NewTestContext("GET", "/metrics/dailies/-1")
	c.Params = gin.Params{{Key: "days", Value: "-1"}}
	MetricsDailies(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 400 {
		t.Fatalf("payload code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}
