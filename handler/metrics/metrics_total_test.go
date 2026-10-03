package metrics

import (
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// assertMetricsTotal 调用 /metrics/total 并断言返回的 reqs/tokens。
// 测试两处（空表 + 有行）复用此形状（check-duplicates 重复率门要求同块 10 行 × ≥2
// 处合并到抽象）。
func assertMetricsTotal(t *testing.T, wantReqs, wantTokens int64, label string) {
	t.Helper()
	c, w := testsupport.NewTestContext("GET", "/metrics/total")
	MetricsTotal(c)
	payload := assertEnvelopeOK[MetricsRes](t, w, label)
	if payload.Data.Reqs != wantReqs || payload.Data.Tokens != wantTokens {
		t.Fatalf("%s: metrics total = %+v, want reqs=%d tokens=%d", label, payload.Data, wantReqs, wantTokens)
	}
}

func TestMetricsTotal_ReturnsStatsTotalOrZero(t *testing.T) {
	testsupport.InitTestDB(t)

	// No row -> zeros
	assertMetricsTotal(t, 0, 0, "no row")

	// Row exists -> return it
	if err := models.DB.Create(&models.StatsTotal{ID: 1, Reqs: 12, Tokens: 345}).Error; err != nil {
		t.Fatalf("create stats total: %v", err)
	}
	assertMetricsTotal(t, 12, 345, "with row")
}
