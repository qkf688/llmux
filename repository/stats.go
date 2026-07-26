package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/atopos31/llmio/models"
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

// StatsRepo 封装六张统计表（StatsTotal / StatsDaily / StatsHourly /
// StatsModelTotal / StatsRealModelTotal / StatsProviderTotal）的数据访问。
// 写侧按语义建模而非按表建模：调用方不需要知道「时间维度统计是三张表」。
type StatsRepo interface {
	// AddTimeBased 按 at 所属日期/小时，向 total/daily/hourly 三表的同一字段累加 delta。
	// delta <= 0 时不做任何写入。三次 upsert 各自独立提交，不包事务：
	// 统计数据允许部分丢失，不值得为它承担事务开销。
	AddTimeBased(ctx context.Context, at time.Time, field TimeStatsField, delta int64) error
	// IncModelCalls 累加模型维度调用次数；name 为空时不写入。
	IncModelCalls(ctx context.Context, name string) error
	// IncRealModelCalls 累加真实模型维度调用次数；name 为空时不写入。
	IncRealModelCalls(ctx context.Context, name string) error
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
	// ListModelCallsDesc 列出模型维度统计，按调用次数降序。
	ListModelCallsDesc(ctx context.Context) ([]models.StatsModelTotal, error)
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

func (r *statsRepo) IncModelCalls(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	return r.incCallsByName(ctx, &models.StatsModelTotal{Name: name, Calls: 1})
}

func (r *statsRepo) IncRealModelCalls(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	return r.incCallsByName(ctx, &models.StatsRealModelTotal{Name: name, Calls: 1})
}

// incCallsByName 按 name 主键 upsert 并自增 calls。
// StatsModelTotal 与 StatsRealModelTotal 表结构同形，共用同一写法。
func (r *statsRepo) incCallsByName(ctx context.Context, row any) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"calls":      gorm.Expr("calls + 1"),
			"updated_at": time.Now(),
		}),
	}).Create(row).Error
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
	if d.ResponseTimeMs > 0 {
		// AvgResponseTime 存的是累计耗时，均值由读侧除以 TotalRequests 得到。
		updates["avg_response_time"] = gorm.Expr("avg_response_time + ?", d.ResponseTimeMs)
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_name"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&models.StatsProviderTotal{
		ProviderName:    d.ProviderName,
		TotalRequests:   1,
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		TotalTokens:     max(d.Tokens, 0),
		AvgResponseTime: max(d.ResponseTimeMs, 0),
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

func (r *statsRepo) ListModelCallsDesc(ctx context.Context) ([]models.StatsModelTotal, error) {
	var rows []models.StatsModelTotal
	if err := r.db.WithContext(ctx).Order("calls DESC").Find(&rows).Error; err != nil {
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
