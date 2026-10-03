package metrics

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/logs"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// assertEnvelopeOK 通用「HTTP 200 + 解码 APIEnvelope + Code=200」三段断言，
// 把 status/Code/unmarshal 三行交给一处管，调用方只关心数据字段。metrics
// handler 测试里多处复用（check-duplicates 重复率门要求同块 10 行 × ≥2
// 处合并到抽象）。
func assertEnvelopeOK[T any](t *testing.T, w *httptest.ResponseRecorder, label string) testsupport.APIEnvelope[T] {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("%s: status = %d, want 200, body=%s", label, w.Code, w.Body.String())
	}
	var payload testsupport.APIEnvelope[T]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("%s: unmarshal: %v, body=%s", label, err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("%s: payload code = %d, want 200", label, payload.Code)
	}
	return payload
}

// assertMetricsUse 调用 /metrics/use/:days 并断言返回的 reqs/tokens。
// before / after clear + days=30 sanity-check 共三处复用此形状。
func assertMetricsUse(t *testing.T, days int, wantReqs, wantTokens int64, label string) {
	t.Helper()
	dayStr := strconv.Itoa(days)
	c, w := testsupport.NewTestContext("GET", "/metrics/use/"+dayStr)
	c.Params = gin.Params{{Key: "days", Value: dayStr}}
	Metrics(c)
	payload := assertEnvelopeOK[MetricsRes](t, w, label)
	if payload.Data.Reqs != wantReqs || payload.Data.Tokens != wantTokens {
		t.Fatalf("%s: metrics = %+v, want reqs=%d tokens=%d", label, payload.Data, wantReqs, wantTokens)
	}
}

// assertRealModelCounts 调用 /metrics/real-model-counts 并断言首条数据。
// before / after clear 两处复用此形状。
func assertRealModelCounts(t *testing.T, wantModel string, wantCalls int64, label string) {
	t.Helper()
	c, w := testsupport.NewTestContext("GET", "/metrics/real-model-counts")
	RealModelCounts(c)
	payload := assertEnvelopeOK[[]Count](t, w, label)
	if len(payload.Data) == 0 || payload.Data[0].Model != wantModel || payload.Data[0].Calls != wantCalls {
		t.Fatalf("%s: real model counts = %+v, want first item %s=%d", label, payload.Data, wantModel, wantCalls)
	}
}

// TestMetricsAndRealModelCounts_NotAffectedByClearAllLogs 端到端验证：
// StatsDaily（时间维度）与 StatsRealModelTotal（真实模型累计）的指标在清空
// chat_logs 后不受影响——这两张累计表由 chatstats 写入路径独立维护，与日志
// 详情表无依赖。StatsModelTotal 已废弃（仪表盘不再按「用户请求模型名」维度
// 独立排行），不再断言。
func TestMetricsAndRealModelCounts_NotAffectedByClearAllLogs(t *testing.T) {
	testsupport.InitTestDB(t)

	today := time.Now().Format("2006-01-02")

	if err := models.DB.Create(&models.StatsDaily{Date: today, Reqs: 7, Tokens: 99}).Error; err != nil {
		t.Fatalf("create stats daily: %v", err)
	}
	if err := models.DB.Create(&models.StatsRealModelTotal{Name: "m1", Calls: 5}).Error; err != nil {
		t.Fatalf("create stats real model total: %v", err)
	}

	// Insert logs so clear endpoints actually delete something.
	log := models.ChatLog{Name: "m1", Status: "success"}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create chat log: %v", err)
	}
	if err := models.DB.Create(&models.ChatIO{LogID: log.ID, Input: "in"}).Error; err != nil {
		t.Fatalf("create chat io: %v", err)
	}

	assertMetricsUse(t, 0, 7, 99, "before clear")
	assertRealModelCounts(t, "m1", 5, "before clear")

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

	assertMetricsUse(t, 0, 7, 99, "after clear")
	assertRealModelCounts(t, "m1", 5, "after clear")

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
	assertMetricsUse(t, 30, 7, 99, "30 days")
}
