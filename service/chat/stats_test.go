package chat

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
)

func TestDisableAllLogs_StillUpdatesStats(t *testing.T) {
	initChatRecordTestDB(t)

	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyDisableAllLogs).
		Update("value", "true").Error; err != nil {
		t.Fatalf("enable disable_all_logs: %v", err)
	}

	logID, err := SaveChatLog(context.Background(), models.ChatLog{
		Name:          "m1",
		ProviderModel: "pm1",
		ProviderName:  "p1",
		Status:        "success",
	})
	if err != nil {
		t.Fatalf("SaveChatLog: %v", err)
	}
	if logID != 0 {
		t.Fatalf("logID = %d, want 0 when disable_all_logs=true", logID)
	}

	var logsCount int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&logsCount).Error; err != nil {
		t.Fatalf("count chat logs: %v", err)
	}
	if logsCount != 0 {
		t.Fatalf("chat logs count = %d, want 0", logsCount)
	}

	today := time.Now().Format("2006-01-02")

	var daily models.StatsDaily
	if err := models.DB.First(&daily, "date = ?", today).Error; err != nil {
		t.Fatalf("load stats daily: %v", err)
	}
	if daily.Reqs != 1 {
		t.Fatalf("daily.reqs = %d, want 1", daily.Reqs)
	}
	if daily.Tokens != 0 {
		t.Fatalf("daily.tokens = %d, want 0 before RecordLog", daily.Tokens)
	}

	var modelTotal models.StatsModelTotal
	if err := models.DB.First(&modelTotal, "name = ?", "m1").Error; err != nil {
		t.Fatalf("load stats model total: %v", err)
	}
	if modelTotal.Calls != 1 {
		t.Fatalf("modelTotal.calls = %d, want 1", modelTotal.Calls)
	}

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		return &models.ChatLog{Usage: models.Usage{TotalTokens: 123}}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
	}

	RecordLog(context.Background(), time.Now(), io.NopCloser(strings.NewReader("x")), proc, 0, Before{Stream: false, raw: []byte(`{}`)}, false)

	var afterDaily models.StatsDaily
	if err := models.DB.First(&afterDaily, "date = ?", today).Error; err != nil {
		t.Fatalf("load stats daily after RecordLog: %v", err)
	}
	if afterDaily.Reqs != 1 {
		t.Fatalf("afterDaily.reqs = %d, want 1", afterDaily.Reqs)
	}
	if afterDaily.Tokens != 123 {
		t.Fatalf("afterDaily.tokens = %d, want 123", afterDaily.Tokens)
	}

	var total models.StatsTotal
	if err := models.DB.First(&total, "id = ?", 1).Error; err != nil {
		t.Fatalf("load stats total: %v", err)
	}
	if total.Reqs != 1 {
		t.Fatalf("total.reqs = %d, want 1", total.Reqs)
	}
	if total.Tokens != 123 {
		t.Fatalf("total.tokens = %d, want 123", total.Tokens)
	}

	var ioCount int64
	if err := models.DB.Model(&models.ChatIO{}).Count(&ioCount).Error; err != nil {
		t.Fatalf("count chat io: %v", err)
	}
	if ioCount != 0 {
		t.Fatalf("chat io count = %d, want 0", ioCount)
	}
}

