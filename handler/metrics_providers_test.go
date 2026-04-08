package handler

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestProviderMetrics_SortsAndComputesRates(t *testing.T) {
	initHandlerTestDB(t)

	p1 := models.Provider{Name: "p1", Type: "openai"}
	p2 := models.Provider{Name: "p2", Type: "openai"}
	if err := models.DB.Create(&p1).Error; err != nil {
		t.Fatalf("create provider p1: %v", err)
	}
	if err := models.DB.Create(&p2).Error; err != nil {
		t.Fatalf("create provider p2: %v", err)
	}

	// 直接写入供应商统计表（模拟请求完成后的增量更新）
	stats := []models.StatsProviderTotal{
		{
			ProviderName:    p1.Name,
			TotalRequests:   3,
			SuccessCount:    2,
			FailureCount:    1,
			TotalTokens:     30,
			AvgResponseTime: 600, // 累计响应时间 (100ms + 300ms + 200ms) = 600ms
		},
		{
			ProviderName:    p2.Name,
			TotalRequests:   2,
			SuccessCount:    2,
			FailureCount:    0,
			TotalTokens:     12,
			AvgResponseTime: 200, // 累计响应时间 (50ms + 150ms) = 200ms
		},
	}
	for i := range stats {
		if err := models.DB.Create(&stats[i]).Error; err != nil {
			t.Fatalf("create stats %d: %v", i, err)
		}
	}

	c, w := newHandlerTestContext("GET", "/metrics/providers")
	ProviderMetrics(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[[]ProviderMetricRes]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}

	if len(payload.Data) != 2 {
		t.Fatalf("providers length = %d, want 2", len(payload.Data))
	}

	// Sorted by total_requests desc: p1(3) then p2(2).
	if payload.Data[0].ProviderName != "p1" || payload.Data[0].TotalRequests != 3 {
		t.Fatalf("first provider = %+v, want p1 with 3 requests", payload.Data[0])
	}
	if payload.Data[1].ProviderName != "p2" || payload.Data[1].TotalRequests != 2 {
		t.Fatalf("second provider = %+v, want p2 with 2 requests", payload.Data[1])
	}

	// Success/failure counts
	if payload.Data[0].SuccessCount != 2 || payload.Data[0].FailureCount != 1 {
		t.Fatalf("p1 success/failure = %d/%d, want 2/1", payload.Data[0].SuccessCount, payload.Data[0].FailureCount)
	}
	if payload.Data[1].SuccessCount != 2 || payload.Data[1].FailureCount != 0 {
		t.Fatalf("p2 success/failure = %d/%d, want 2/0", payload.Data[1].SuccessCount, payload.Data[1].FailureCount)
	}

	// Avg response time: p1 avg=600/3=200ms; p2 avg=200/2=100ms.
	wantP1 := int64(200)
	wantP2 := int64(100)
	if payload.Data[0].AvgResponseTime != wantP1 {
		t.Fatalf("p1 avg_response_time=%d, want %d", payload.Data[0].AvgResponseTime, wantP1)
	}
	if payload.Data[1].AvgResponseTime != wantP2 {
		t.Fatalf("p2 avg_response_time=%d, want %d", payload.Data[1].AvgResponseTime, wantP2)
	}

	// Total tokens should match stats.
	if payload.Data[0].TotalTokens != 30 {
		t.Fatalf("p1 total_tokens=%d, want 30", payload.Data[0].TotalTokens)
	}
	if payload.Data[1].TotalTokens != 12 {
		t.Fatalf("p2 total_tokens=%d, want 12", payload.Data[1].TotalTokens)
	}
}
