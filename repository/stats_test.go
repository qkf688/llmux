package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.New(t,
		&models.StatsTotal{},
		&models.StatsDaily{},
		&models.StatsHourly{},
		&models.StatsRealModelTotal{},
		&models.StatsProviderTotal{},
	)
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
	// 只有 2 次成功请求有耗时，失败请求不计入样本数。
	if p1.ResponseTimeSamples != 2 {
		t.Fatalf("p1 response_time_samples = %d, want 2 (only successful requests with response time)", p1.ResponseTimeSamples)
	}
}

// realModelRow 读单行真实模型统计，空表时返回 nil。
func realModelRow(t *testing.T, ctx context.Context, repo StatsRepo, name string) *models.StatsRealModelTotal {
	t.Helper()
	rows, err := repo.ListRealModelCallsDesc(ctx)
	if err != nil {
		t.Fatalf("list real model totals: %v", err)
	}
	for i := range rows {
		if rows[i].Name == name {
			return &rows[i]
		}
	}
	return nil
}

func TestStatsRepo_AddRealModelTokens_AccumulatesAlongsideCalls(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	// 先建行计 calls（建行时刻），再结算 token（结算时刻）——生产路径的先后顺序。
	if err := repo.IncRealModelCalls(ctx, "m1"); err != nil {
		t.Fatalf("inc calls: %v", err)
	}
	if err := repo.AddRealModelTokens(ctx, RealModelTokenDelta{
		Name: "m1", PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120,
	}); err != nil {
		t.Fatalf("add tokens: %v", err)
	}
	if err := repo.AddRealModelTokens(ctx, RealModelTokenDelta{
		Name: "m1", PromptTokens: 50, CompletionTokens: 10, TotalTokens: 60,
	}); err != nil {
		t.Fatalf("add tokens again: %v", err)
	}

	row := realModelRow(t, ctx, repo, "m1")
	if row == nil {
		t.Fatal("m1 row missing, want calls and tokens on the same row")
	}
	if row.TotalTokens != 180 || row.PromptTokens != 150 || row.CompletionTokens != 30 {
		t.Fatalf("tokens = %d/%d/%d, want 180/150/30",
			row.TotalTokens, row.PromptTokens, row.CompletionTokens)
	}
	// token 写入走独立 upsert，若它整行覆盖过会把已记的 calls 抹成 0。
	if row.Calls != 1 {
		t.Fatalf("calls = %d, want 1（token 累加不得覆盖 calls）", row.Calls)
	}
}

func TestStatsRepo_AddRealModelTokens_RejectsNoOp(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	// 空模型名不建行。
	if err := repo.AddRealModelTokens(ctx, RealModelTokenDelta{TotalTokens: 100}); err != nil {
		t.Fatalf("empty name should be a no-op, got %v", err)
	}
	// 失败请求无 usage，三列全 0：建行只会造出「有行但恒零」的假账，
	// 读侧无法区分「真没消耗」与「没上报 usage」。
	if err := repo.AddRealModelTokens(ctx, RealModelTokenDelta{Name: "m1"}); err != nil {
		t.Fatalf("all-zero delta should be a no-op, got %v", err)
	}
	if row := realModelRow(t, ctx, repo, "m1"); row != nil {
		t.Fatalf("row = %+v, want no row written", row)
	}
}

func TestStatsRepo_AddRealModelTokens_AccumulatesOnlyPositiveColumns(t *testing.T) {
	ctx := context.Background()
	repo := NewStatsRepo(newStatsTestDB(t))

	// 上游只回 total 未拆分输入输出时，拆分两列不应被写成 0 以外的任何值。
	if err := repo.AddRealModelTokens(ctx, RealModelTokenDelta{Name: "m1", TotalTokens: 500}); err != nil {
		t.Fatalf("add total only: %v", err)
	}
	row := realModelRow(t, ctx, repo, "m1")
	if row == nil || row.TotalTokens != 500 || row.PromptTokens != 0 || row.CompletionTokens != 0 {
		t.Fatalf("row = %+v, want total 500 with split columns untouched", row)
	}
	// 负数（上游脏 usage）不得倒扣已累计值。
	if err := repo.AddRealModelTokens(ctx, RealModelTokenDelta{
		Name: "m1", PromptTokens: -1, CompletionTokens: -1, TotalTokens: -1,
	}); err != nil {
		t.Fatalf("negative delta should be skipped, got %v", err)
	}
	row = realModelRow(t, ctx, repo, "m1")
	if row.TotalTokens != 500 {
		t.Fatalf("total = %d, want 500（负增量不得倒扣）", row.TotalTokens)
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
