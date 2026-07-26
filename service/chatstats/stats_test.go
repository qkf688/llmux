package chatstats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
)

func initStatsTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	// 与 models.DB 同步默认 Repositories，避免 Default 缓存上一个用例的连接
	repository.SetDefault(repository.New(models.DB))
	t.Cleanup(func() {
		repository.SetDefault(nil)
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

func TestRecordRequestStatsAndTokenStats_UpdatesHourly(t *testing.T) {
	initStatsTestDB(t)

	at := time.Date(2026, 1, 2, 15, 4, 5, 0, time.Now().Location())
	date := at.Format("2006-01-02")
	hour := at.Hour()

	if err := RecordRequestStats(context.Background(), at, ""); err != nil {
		t.Fatalf("RecordRequestStats: %v", err)
	}
	if err := RecordRequestStats(context.Background(), at, ""); err != nil {
		t.Fatalf("RecordRequestStats again: %v", err)
	}
	if err := RecordTokenStats(context.Background(), at, 100); err != nil {
		t.Fatalf("RecordTokenStats: %v", err)
	}
	if err := RecordTokenStats(context.Background(), at, 23); err != nil {
		t.Fatalf("RecordTokenStats again: %v", err)
	}

	if err := RecordRealModelRequestStats(context.Background(), "rm1"); err != nil {
		t.Fatalf("RecordRealModelRequestStats: %v", err)
	}
	if err := RecordRealModelRequestStats(context.Background(), "rm1"); err != nil {
		t.Fatalf("RecordRealModelRequestStats again: %v", err)
	}

	var realModelTotal models.StatsRealModelTotal
	if err := models.DB.First(&realModelTotal, "name = ?", "rm1").Error; err != nil {
		t.Fatalf("load stats real model total: %v", err)
	}
	if realModelTotal.Calls != 2 {
		t.Fatalf("realModelTotal.calls=%d, want 2", realModelTotal.Calls)
	}

	var hourly models.StatsHourly
	if err := models.DB.First(&hourly, "date = ? AND hour = ?", date, hour).Error; err != nil {
		t.Fatalf("load stats hourly: %v", err)
	}
	if hourly.Reqs != 2 {
		t.Fatalf("hourly.reqs=%d, want 2", hourly.Reqs)
	}
	if hourly.Tokens != 123 {
		t.Fatalf("hourly.tokens=%d, want 123", hourly.Tokens)
	}
}
