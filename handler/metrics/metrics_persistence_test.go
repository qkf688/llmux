package metrics

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/atopos31/llmio/handler/logs"
	"github.com/atopos31/llmio/handler/testsupport"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

func TestMetricsAndCounts_NotAffectedByClearAllLogs(t *testing.T) {
	testsupport.InitTestDB(t)

	today := time.Now().Format("2006-01-02")

	if err := models.DB.Create(&models.StatsDaily{Date: today, Reqs: 7, Tokens: 99}).Error; err != nil {
		t.Fatalf("create stats daily: %v", err)
	}
	if err := models.DB.Create(&models.StatsModelTotal{Name: "m1", Calls: 42}).Error; err != nil {
		t.Fatalf("create stats model total: %v", err)
	}
	if err := models.DB.Create(&models.StatsRealModelTotal{Name: "m1", Calls: 5}).Error; err != nil {
		t.Fatalf("create stats real model total: %v", err)
	}

	// Insert logs so clear endpoints actually delete something.
	log := models.ChatLog{Name: "m1", Status: "success"}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create chat log: %v", err)
	}
	if err := models.DB.Create(&models.ChatIO{LogId: log.ID, Input: "in"}).Error; err != nil {
		t.Fatalf("create chat io: %v", err)
	}

	// metrics before clear
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/use/0")
		c.Params = gin.Params{{Key: "days", Value: "0"}}
		Metrics(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var payload testsupport.APIEnvelope[MetricsRes]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if payload.Data.Reqs != 7 || payload.Data.Tokens != 99 {
			t.Fatalf("metrics = %+v, want reqs=7 tokens=99", payload.Data)
		}
	}

	// counts before clear
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/counts")
		Counts(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload testsupport.APIEnvelope[[]Count]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if len(payload.Data) == 0 || payload.Data[0].Model != "m1" || payload.Data[0].Calls != 42 {
			t.Fatalf("counts = %+v, want first item m1=42", payload.Data)
		}
	}

	// real model counts before clear
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/real-model-counts")
		RealModelCounts(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload testsupport.APIEnvelope[[]Count]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if len(payload.Data) == 0 || payload.Data[0].Model != "m1" || payload.Data[0].Calls != 5 {
			t.Fatalf("real model counts = %+v, want first item m1=5", payload.Data)
		}
	}

	// clear logs
	{
		c, w := testsupport.NewTestContext("DELETE", "/logs/clear")
		logs.ClearAllLogs(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload testsupport.APIEnvelope[map[string]any]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if deletedRaw, ok := payload.Data["deleted"]; !ok {
			t.Fatalf("expected deleted in response, got %+v", payload.Data)
		} else if deleted, ok := deletedRaw.(float64); !ok || deleted < 1 {
			t.Fatalf("deleted = %v, want >= 1", deletedRaw)
		}
	}

	// metrics after clear
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/use/0")
		c.Params = gin.Params{{Key: "days", Value: "0"}}
		Metrics(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var payload testsupport.APIEnvelope[MetricsRes]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if payload.Data.Reqs != 7 || payload.Data.Tokens != 99 {
			t.Fatalf("metrics after clear = %+v, want reqs=7 tokens=99", payload.Data)
		}
	}

	// counts after clear
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/counts")
		Counts(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload testsupport.APIEnvelope[[]Count]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if len(payload.Data) == 0 || payload.Data[0].Model != "m1" || payload.Data[0].Calls != 42 {
			t.Fatalf("counts after clear = %+v, want first item m1=42", payload.Data)
		}
	}

	// real model counts after clear
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/real-model-counts")
		RealModelCounts(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload testsupport.APIEnvelope[[]Count]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if len(payload.Data) == 0 || payload.Data[0].Model != "m1" || payload.Data[0].Calls != 5 {
			t.Fatalf("real model counts after clear = %+v, want first item m1=5", payload.Data)
		}
	}

	// Ensure the log tables are actually cleared.
	{
		var logsCount int64
		if err := models.DB.Model(&models.ChatLog{}).Count(&logsCount).Error; err != nil {
			t.Fatalf("count chat logs: %v", err)
		}
		if logsCount != 0 {
			t.Fatalf("chat logs count = %d, want 0", logsCount)
		}

		var ioCount int64
		if err := models.DB.Model(&models.ChatIO{}).Count(&ioCount).Error; err != nil {
			t.Fatalf("count chat io: %v", err)
		}
		if ioCount != 0 {
			t.Fatalf("chat io count = %d, want 0", ioCount)
		}
	}

	// Also sanity-check Metrics with days=30 uses the same source (stats_dailies).
	{
		c, w := testsupport.NewTestContext("GET", "/metrics/use/30")
		c.Params = gin.Params{{Key: "days", Value: strconv.Itoa(30)}}
		Metrics(c)
		if w.Code != 200 {
			t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload testsupport.APIEnvelope[MetricsRes]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
		if payload.Data.Reqs != 7 || payload.Data.Tokens != 99 {
			t.Fatalf("metrics 30 days = %+v, want reqs=7 tokens=99", payload.Data)
		}
	}
}
