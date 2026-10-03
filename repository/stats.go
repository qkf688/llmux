package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// statsDateLayout 统计表日期列的存储格式。
// 该格式属于存储布局细节，只在本包内出现；调用方一律传 time.Time。
const statsDateLayout = "2006-01-02"

// TimeStatsField 时间维度统计（total/daily/hourly 三表共用列）的可累加字段。
type TimeStatsField string

const (
	StatsFieldReqs   TimeStatsField = "reqs"
	StatsFieldTokens TimeStatsField = "tokens"
)

// StatsSum 时间维度统计的聚合结果。
type StatsSum struct {
	Reqs   int64
	Tokens int64
}

// ProviderStatsDelta 供应商维度的一次增量。
// ResponseTimeMs / Tokens 为 0 或负数时对应列不累加。
type ProviderStatsDelta struct {
	ProviderName   string
	Success        bool
	ResponseTimeMs int64
	Tokens         int64
}

// RealModelTokenDelta 真实模型维度的一次 token 增量。
// 与 ProviderStatsDelta.Tokens 单列不同，这里分列携带输入/输出：
// 读侧要展示输入输出构成，且两者的比值是识别上游 usage 归集异常
// （输入远小于输出等）的唯一依据，压成总数就再也拆不开了。
type RealModelTokenDelta struct {
	Name             string
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// StatsRepo 封装四张统计表（StatsTotal / StatsDaily / StatsHourly /
// StatsProviderTotal）的数据访问。StatsModelTotal 已废弃（仪表盘不再按
// 「用户请求模型名」维度独立排行，详见 models/migrate.go 的 dropLegacyStatsModelTable）。
// 写侧按语义建模而非按表建模：调用方不需要知道「时间维度统计是三张表」。
type StatsRepo interface {
	// AddTimeBased 按 at 所属日期/小时，向 total/daily/hourly 三表的同一字段累加 delta。
	// delta <= 0 时不做任何写入。三次 upsert 各自独立提交，不包事务：
	// 统计数据允许部分丢失，不值得为它承担事务开销。
	AddTimeBased(ctx context.Context, at time.Time, field TimeStatsField, delta int64) error
	// IncRealModelCalls 累加真实模型维度调用次数；name 为空时不写入。
	// 建行时刻（SaveChatLog）调用：那刻只有模型名，token 尚不可知。
	IncRealModelCalls(ctx context.Context, name string) error
	// AddRealModelTokens 累加真实模型维度 token；name 为空或三列全为 0 时不写入
	// （失败请求没有 usage，写 0 行只会让读侧把「未知」误显成「零消耗」）。
	// 结算时刻（chat.RecordLog）调用，与 IncRealModelCalls 分属不同时点。
	AddRealModelTokens(ctx context.Context, d RealModelTokenDelta) error
	// AddProviderStats 累加供应商维度请求/成功失败/token/累计响应时间；ProviderName 为空时不写入。
	AddProviderStats(ctx context.Context, d ProviderStatsDelta) error

	// GetTotal 读取全量累计行（固定 ID=1）；无行时返回 gorm.ErrRecordNotFound，由调用方决定降级。
	GetTotal(ctx context.Context) (*models.StatsTotal, error)
	// SumDailiesSince 汇总 since 当日（含）之后的日统计；无数据时返回零值而非错误。
	SumDailiesSince(ctx context.Context, since time.Time) (StatsSum, error)
	// ListDailiesSince 列出 since 当日（含）之后的日统计，按日期升序。
	ListDailiesSince(ctx context.Context, since time.Time) ([]models.StatsDaily, error)
	// ListHourliesByDate 列出指定日期的小时统计，按小时升序；缺失的小时不补零，由调用方处理。
	ListHourliesByDate(ctx context.Context, day time.Time) ([]models.StatsHourly, error)
	// ListRealModelCallsDesc 列出真实模型维度统计，按调用次数降序。
	ListRealModelCallsDesc(ctx context.Context) ([]models.StatsRealModelTotal, error)
	// ListProviderTotalsDesc 列出供应商维度统计，按请求总数降序；比率计算由调用方完成。
	ListProviderTotalsDesc(ctx context.Context) ([]models.StatsProviderTotal, error)
}

// NewStatsRepo 创建 StatsRepo。
func NewStatsRepo(db *gorm.DB) StatsRepo {
	return &statsRepo{db: db}
}

type statsRepo struct {
	db *gorm.DB
}

func (r *statsRepo) AddTimeBased(ctx context.Context, at time.Time, field TimeStatsField, delta int64) error {
	if delta <= 0 {
		return nil
	}
	switch field {
	case StatsFieldReqs, StatsFieldTokens:
	default:
		return fmt.Errorf("repository: unknown time stats field %q", field)
	}

	db := r.db.WithContext(ctx)
	now := time.Now()
	date := at.Format(statsDateLayout)
	hour := at.Hour()

	col := string(field)
	// OnConflict 路径下 GORM 不自动维护 updated_at，须显式赋值。
	updates := clause.Assignments(map[string]any{
		col:          gorm.Expr(col+" + ?", delta),
		"updated_at": now,
	})

	total := &models.StatsTotal{ID: 1}
	daily := &models.StatsDaily{Date: date}
	hourly := &models.StatsHourly{Date: date, Hour: hour}
	setTimeStatsField(field, delta, &total.Reqs, &total.Tokens)
	setTimeStatsField(field, delta, &daily.Reqs, &daily.Tokens)
	setTimeStatsField(field, delta, &hourly.Reqs, &hourly.Tokens)

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: updates,
	}).Create(total).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: updates,
	}).Create(daily).Error; err != nil {
		return err
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "hour"}},
		DoUpdates: updates,
	}).Create(hourly).Error
}

// setTimeStatsField 把 delta 写进三张时间统计表首次插入时对应的列。
// 三张表列名一致，故只按字段分派指针，避免为每张表重复一遍 switch。
func setTimeStatsField(field TimeStatsField, delta int64, reqs *int64, tokens *int64) {
	switch field {
	case StatsFieldReqs:
		*reqs = delta
	case StatsFieldTokens:
		*tokens = delta
	}
}

func (r *statsRepo) IncRealModelCalls(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	return r.incCallsByName(ctx, &models.StatsRealModelTotal{Name: name, Calls: 1})
}

// incCallsByName 按 name 主键 upsert 并自增 calls。
// StatsRealModelTotal 单表复用本写法。
func (r *statsRepo) incCallsByName(ctx context.Context, row any) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"calls":      gorm.Expr("calls + 1"),
			"updated_at": time.Now(),
		}),
	}).Create(row).Error
}

// AddRealModelTokens 累加真实模型维度 token 三列。只累加实际有值的列——
// 失败请求没有 usage，其三列全 0；此时若照写会造出一行 calls 有值、token 恒 0
// 的记录，读侧无法区分「这次请求真没消耗」与「这次请求没上报 usage」。
func (r *statsRepo) AddRealModelTokens(ctx context.Context, d RealModelTokenDelta) error {
	if d.Name == "" {
		return nil
	}
	updates := map[string]any{"updated_at": time.Now()}
	if d.PromptTokens > 0 {
		updates["prompt_tokens"] = gorm.Expr("prompt_tokens + ?", d.PromptTokens)
	}
	if d.CompletionTokens > 0 {
		updates["completion_tokens"] = gorm.Expr("completion_tokens + ?", d.CompletionTokens)
	}
	if d.TotalTokens > 0 {
		updates["total_tokens"] = gorm.Expr("total_tokens + ?", d.TotalTokens)
	}
	if len(updates) == 1 {
		return nil
	}

	// Create 侧必须带真实初值：ON CONFLICT DO UPDATE 只在主键冲突时执行，
	// 没有 calls 行的新模型走的是纯 INSERT，此时写进去的是 Create 的值——
	// 漏带就会把这次 token 记成 0（DoUpdates 里的累加表达式对新行不参与）。
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&models.StatsRealModelTotal{
		Name:             d.Name,
		PromptTokens:     max(d.PromptTokens, 0),
		CompletionTokens: max(d.CompletionTokens, 0),
		TotalTokens:      max(d.TotalTokens, 0),
	}).Error
}

func (r *statsRepo) AddProviderStats(ctx context.Context, d ProviderStatsDelta) error {
	if d.ProviderName == "" {
		return nil
	}

	updates := map[string]any{
		"total_requests": gorm.Expr("total_requests + 1"),
		"updated_at":     time.Now(),
	}

	var successCount, failureCount int64
	if d.Success {
		successCount = 1
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		failureCount = 1
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	if d.Tokens > 0 {
		updates["total_tokens"] = gorm.Expr("total_tokens + ?", d.Tokens)
	}
	var responseTimeSamples int64
	if d.ResponseTimeMs > 0 {
		// AvgResponseTime 存的是累计耗时，均值由读侧除以 ResponseTimeSamples 得到。
		// 只有实际有耗时的请求才计入样本，避免失败请求（ResponseTimeMs=0）稀释均值。
		updates["avg_response_time"] = gorm.Expr("avg_response_time + ?", d.ResponseTimeMs)
		updates["response_time_samples"] = gorm.Expr("response_time_samples + 1")
		responseTimeSamples = 1
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_name"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&models.StatsProviderTotal{
		ProviderName:        d.ProviderName,
		TotalRequests:       1,
		SuccessCount:        successCount,
		FailureCount:        failureCount,
		TotalTokens:         max(d.Tokens, 0),
		AvgResponseTime:     max(d.ResponseTimeMs, 0),
		ResponseTimeSamples: responseTimeSamples,
	}).Error
}

func (r *statsRepo) GetTotal(ctx context.Context) (*models.StatsTotal, error) {
	var total models.StatsTotal
	if err := r.db.WithContext(ctx).
		Model(&models.StatsTotal{}).
		Select("reqs", "tokens").
		Where("id = ?", 1).
		Take(&total).Error; err != nil {
		return nil, err
	}
	return &total, nil
}

func (r *statsRepo) SumDailiesSince(ctx context.Context, since time.Time) (StatsSum, error) {
	var sum StatsSum
	// 空结果集下 SUM 返回 NULL，COALESCE 兜底成 0，避免 Scan 报错。
	err := r.db.WithContext(ctx).
		Model(&models.StatsDaily{}).
		Select("COALESCE(SUM(reqs), 0) AS reqs, COALESCE(SUM(tokens), 0) AS tokens").
		Where("date >= ?", since.Format(statsDateLayout)).
		Scan(&sum).Error
	if err != nil {
		return StatsSum{}, err
	}
	return sum, nil
}

func (r *statsRepo) ListDailiesSince(ctx context.Context, since time.Time) ([]models.StatsDaily, error) {
	var rows []models.StatsDaily
	if err := r.db.WithContext(ctx).
		Where("date >= ?", since.Format(statsDateLayout)).
		Order("date ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *statsRepo) ListHourliesByDate(ctx context.Context, day time.Time) ([]models.StatsHourly, error) {
	var rows []models.StatsHourly
	if err := r.db.WithContext(ctx).
		Where("date = ?", day.Format(statsDateLayout)).
		Order("hour ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *statsRepo) ListRealModelCallsDesc(ctx context.Context) ([]models.StatsRealModelTotal, error) {
	var rows []models.StatsRealModelTotal
	if err := r.db.WithContext(ctx).Order("calls DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *statsRepo) ListProviderTotalsDesc(ctx context.Context) ([]models.StatsProviderTotal, error) {
	var rows []models.StatsProviderTotal
	if err := r.db.WithContext(ctx).Order("total_requests DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
