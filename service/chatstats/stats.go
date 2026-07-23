package chatstats

import (
	"context"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordRequestStats 累加请求次数（total/daily/hourly）及可选模型维度 calls。
func RecordRequestStats(ctx context.Context, at time.Time, modelName string) error {
	db := models.DB.WithContext(ctx)
	now := time.Now()
	date := at.Format("2006-01-02")
	hour := at.Hour()

	if err := upsertTimeBasedStats(db, now, date, hour, "reqs", gorm.Expr("reqs + 1"),
		func() *models.StatsTotal { return &models.StatsTotal{ID: 1, Reqs: 1} },
		func() *models.StatsDaily { return &models.StatsDaily{Date: date, Reqs: 1} },
		func() *models.StatsHourly { return &models.StatsHourly{Date: date, Hour: hour, Reqs: 1} },
	); err != nil {
		return err
	}

	if modelName != "" {
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}},
			DoUpdates: clause.Assignments(map[string]any{
				"calls":      gorm.Expr("calls + 1"),
				"updated_at": now,
			}),
		}).Create(&models.StatsModelTotal{Name: modelName, Calls: 1}).Error; err != nil {
			return err
		}
	}

	return nil
}

// RecordRealModelRequestStats 累加真实模型维度 calls。
func RecordRealModelRequestStats(ctx context.Context, at time.Time, realModelName string) error {
	if realModelName == "" {
		return nil
	}

	db := models.DB.WithContext(ctx)
	now := time.Now()

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"calls":      gorm.Expr("calls + 1"),
			"updated_at": now,
		}),
	}).Create(&models.StatsRealModelTotal{Name: realModelName, Calls: 1}).Error; err != nil {
		return err
	}

	return nil
}

// RecordProviderStats 累加供应商维度请求/成功失败/token/响应时间。
func RecordProviderStats(ctx context.Context, providerName string, success bool, responseTimeMs int64, tokens int64) error {
	if providerName == "" {
		return nil
	}

	db := models.DB.WithContext(ctx)
	now := time.Now()

	updates := map[string]any{
		"total_requests": gorm.Expr("total_requests + 1"),
		"updated_at":     now,
	}

	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	if tokens > 0 {
		updates["total_tokens"] = gorm.Expr("total_tokens + ?", tokens)
	}

	if responseTimeMs > 0 {
		updates["avg_response_time"] = gorm.Expr("avg_response_time + ?", responseTimeMs)
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_name"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&models.StatsProviderTotal{
		ProviderName:  providerName,
		TotalRequests: 1,
		SuccessCount: func() int64 {
			if success {
				return 1
			}
			return 0
		}(),
		FailureCount: func() int64 {
			if !success {
				return 1
			}
			return 0
		}(),
		TotalTokens:     tokens,
		AvgResponseTime: responseTimeMs,
	}).Error; err != nil {
		return err
	}

	return nil
}

// RecordTokenStats 累加 token 次数（total/daily/hourly）。
func RecordTokenStats(ctx context.Context, at time.Time, tokens int64) error {
	if tokens <= 0 {
		return nil
	}

	db := models.DB.WithContext(ctx)
	now := time.Now()
	date := at.Format("2006-01-02")
	hour := at.Hour()

	return upsertTimeBasedStats(db, now, date, hour, "tokens", gorm.Expr("tokens + ?", tokens),
		func() *models.StatsTotal { return &models.StatsTotal{ID: 1, Tokens: tokens} },
		func() *models.StatsDaily { return &models.StatsDaily{Date: date, Tokens: tokens} },
		func() *models.StatsHourly { return &models.StatsHourly{Date: date, Hour: hour, Tokens: tokens} },
	)
}

// upsertTimeBasedStats 处理 StatsTotal、StatsDaily、StatsHourly 三个时间维度表的 upsert 逻辑。
// makeTotal/makeDaily/makeHourly 构造首次插入的记录指针（含初始值），updateExpr 为冲突时的更新表达式。
func upsertTimeBasedStats(
	db *gorm.DB, now time.Time, date string, hour int,
	fieldName string, updateExpr clause.Expr,
	makeTotal func() *models.StatsTotal,
	makeDaily func() *models.StatsDaily,
	makeHourly func() *models.StatsHourly,
) error {
	updates := clause.Assignments(map[string]any{
		fieldName:    updateExpr,
		"updated_at": now,
	})

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: updates,
	}).Create(makeTotal()).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: updates,
	}).Create(makeDaily()).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "hour"}},
		DoUpdates: updates,
	}).Create(makeHourly()).Error; err != nil {
		return err
	}

	return nil
}
