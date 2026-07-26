package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(
		&models.StatsTotal{},
		&models.StatsDaily{},
		&models.StatsHourly{},
		&models.StatsModelTotal{},
		&models.StatsRealModelTotal{},
		&models.StatsProviderTotal{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// 固定时刻，避免依赖 time.Now 导致跨零点 flaky。
var (
	statsAt     = time.Date(2026, 7, 26, 14, 30, 0, 0, time.UTC)
	statsAtDate = "2026-07-26"
)

func TestStatsRepo_AddTimeBased_AccumulatesAcrossThreeTables(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	for range 2 {
		if err := repo.AddTimeBased(ctx, statsAt, StatsFieldReqs, 1); err != nil {
			t.Fatalf("add reqs: %v", err)
		}
	}
	if err := repo.AddTimeBased(ctx, statsAt, StatsFieldTokens, 30); err != nil {
		t.Fatalf("add tokens: %v", err)
	}

	total, err := repo.GetTotal(ctx)
	if err != nil {
		t.Fatalf("get total: %v", err)
	}
	if total.Reqs != 2 || total.Tokens != 30 {
		t.Fatalf("total = %d reqs / %d tokens, want 2/30", total.Reqs, total.Tokens)
	}

	dailies, err := repo.ListDailiesSince(ctx, statsAt)
	if err != nil {
		t.Fatalf("list dailies: %v", err)
	}
	if len(dailies) != 1 || dailies[0].Date != statsAtDate || dailies[0].Reqs != 2 || dailies[0].Tokens != 30 {
		t.Fatalf("dailies = %+v, want single row 2/30 at %s", dailies, statsAtDate)
	}

	hourlies, err := repo.ListHourliesByDate(ctx, statsAt)
	if err != nil {
		t.Fatalf("list hourlies: %v", err)
	}
	if len(hourlies) != 1 || hourlies[0].Hour != 14 || hourlies[0].Reqs != 2 || hourlies[0].Tokens != 30 {
		t.Fatalf("hourlies = %+v, want single row at hour 14 with 2/30", hourlies)
	}
}

func TestStatsRepo_AddTimeBased_RejectsBadInput(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	if err := repo.AddTimeBased(ctx, statsAt, StatsFieldTokens, 0); err != nil {
		t.Fatalf("delta 0 should be a no-op, got %v", err)
	}
	if _, err := repo.GetTotal(ctx); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected no row written, got err = %v", err)
	}

	if err := repo.AddTimeBased(ctx, statsAt, TimeStatsField("bogus"), 1); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestStatsRepo_GetTotal_NotFoundOnEmpty(t *testing.T) {
	repo := NewStatsRepo(newStatsTestDB(t))
	if _, err := repo.GetTotal(context.Background()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want ErrRecordNotFound", err)
	}
}

func TestStatsRepo_IncCalls_AccumulatesAndSortsDesc(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	if err := repo.IncModelCalls(ctx, ""); err != nil {
		t.Fatalf("empty name should be a no-op, got %v", err)
	}
	for range 3 {
		if err := repo.IncModelCalls(ctx, "gpt"); err != nil {
			t.Fatalf("inc model: %v", err)
		}
	}
	if err := repo.IncModelCalls(ctx, "claude"); err != nil {
		t.Fatalf("inc model: %v", err)
	}
	for range 2 {
		if err := repo.IncRealModelCalls(ctx, "real-a"); err != nil {
			t.Fatalf("inc real model: %v", err)
		}
	}

	got, err := repo.ListModelCallsDesc(ctx)
	if err != nil {
		t.Fatalf("list model calls: %v", err)
	}
	if len(got) != 2 || got[0].Name != "gpt" || got[0].Calls != 3 || got[1].Name != "claude" || got[1].Calls != 1 {
		t.Fatalf("model calls = %+v, want gpt:3 then claude:1", got)
	}

	real, err := repo.ListRealModelCallsDesc(ctx)
	if err != nil {
		t.Fatalf("list real model calls: %v", err)
	}
	if len(real) != 1 || real[0].Name != "real-a" || real[0].Calls != 2 {
		t.Fatalf("real model calls = %+v, want real-a:2", real)
	}
}

func TestStatsRepo_AddProviderStats(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	if err := repo.AddProviderStats(ctx, ProviderStatsDelta{Success: true}); err != nil {
		t.Fatalf("empty provider name should be a no-op, got %v", err)
	}

	// 成功两次（带 token 与耗时）+ 失败一次（无 token 无耗时，不应污染累计列）。
	for range 2 {
		if err := repo.AddProviderStats(ctx, ProviderStatsDelta{
			ProviderName: "p1", Success: true, ResponseTimeMs: 200, Tokens: 10,
		}); err != nil {
			t.Fatalf("add provider stats: %v", err)
		}
	}
	if err := repo.AddProviderStats(ctx, ProviderStatsDelta{ProviderName: "p1"}); err != nil {
		t.Fatalf("add provider stats: %v", err)
	}
	if err := repo.AddProviderStats(ctx, ProviderStatsDelta{ProviderName: "p2", Success: true}); err != nil {
		t.Fatalf("add provider stats: %v", err)
	}

	rows, err := repo.ListProviderTotalsDesc(ctx)
	if err != nil {
		t.Fatalf("list provider totals: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].ProviderName != "p1" || rows[1].ProviderName != "p2" {
		t.Fatalf("order = %s,%s, want p1,p2 (by total_requests desc)", rows[0].ProviderName, rows[1].ProviderName)
	}
	p1 := rows[0]
	if p1.TotalRequests != 3 || p1.SuccessCount != 2 || p1.FailureCount != 1 {
		t.Fatalf("p1 counts = %d/%d/%d, want 3/2/1", p1.TotalRequests, p1.SuccessCount, p1.FailureCount)
	}
	if p1.TotalTokens != 20 || p1.AvgResponseTime != 400 {
		t.Fatalf("p1 tokens/cumulative-rt = %d/%d, want 20/400", p1.TotalTokens, p1.AvgResponseTime)
	}
}

func TestStatsRepo_ReadsFilterByDate(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	older := statsAt.AddDate(0, 0, -2)
	if err := repo.AddTimeBased(ctx, older, StatsFieldReqs, 5); err != nil {
		t.Fatalf("add older: %v", err)
	}
	if err := repo.AddTimeBased(ctx, statsAt, StatsFieldReqs, 7); err != nil {
		t.Fatalf("add today: %v", err)
	}

	// since 取当日：边界为闭区间，只应命中当天。
	sum, err := repo.SumDailiesSince(ctx, statsAt)
	if err != nil {
		t.Fatalf("sum dailies: %v", err)
	}
	if sum.Reqs != 7 {
		t.Fatalf("sum since today = %d, want 7", sum.Reqs)
	}

	sum, err = repo.SumDailiesSince(ctx, older)
	if err != nil {
		t.Fatalf("sum dailies: %v", err)
	}
	if sum.Reqs != 12 {
		t.Fatalf("sum since older = %d, want 12", sum.Reqs)
	}

	dailies, err := repo.ListDailiesSince(ctx, older)
	if err != nil {
		t.Fatalf("list dailies: %v", err)
	}
	if len(dailies) != 2 || dailies[0].Date >= dailies[1].Date {
		t.Fatalf("dailies = %+v, want 2 rows ascending by date", dailies)
	}

	// 空表/无匹配行时聚合返回零值而非报错。
	future := statsAt.AddDate(0, 0, 10)
	sum, err = repo.SumDailiesSince(ctx, future)
	if err != nil {
		t.Fatalf("sum dailies on empty range: %v", err)
	}
	if sum.Reqs != 0 || sum.Tokens != 0 {
		t.Fatalf("sum = %+v, want zero value", sum)
	}

	hourlies, err := repo.ListHourliesByDate(ctx, future)
	if err != nil {
		t.Fatalf("list hourlies on empty date: %v", err)
	}
	if len(hourlies) != 0 {
		t.Fatalf("hourlies = %+v, want empty", hourlies)
	}
}
