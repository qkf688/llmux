package chat

import (
	"context"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func recordRequestStats(ctx context.Context, at time.Time, modelName string) error {
	db := models.DB.WithContext(ctx)
	now := time.Now()
	date := at.Format("2006-01-02")
	hour := at.Hour()

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"reqs":       gorm.Expr("reqs + 1"),
			"updated_at": now,
		}),
	}).Create(&models.StatsTotal{ID: 1, Reqs: 1}).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"reqs":       gorm.Expr("reqs + 1"),
			"updated_at": now,
		}),
	}).Create(&models.StatsDaily{Date: date, Reqs: 1}).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}, {Name: "hour"}},
		DoUpdates: clause.Assignments(map[string]any{
			"reqs":       gorm.Expr("reqs + 1"),
			"updated_at": now,
		}),
	}).Create(&models.StatsHourly{Date: date, Hour: hour, Reqs: 1}).Error; err != nil {
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

func recordRealModelRequestStats(ctx context.Context, at time.Time, realModelName string) error {
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

func recordProviderStats(ctx context.Context, providerName string, success bool, responseTimeMs int64, tokens int64) error {
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

func recordTokenStats(ctx context.Context, at time.Time, tokens int64) error {
	if tokens <= 0 {
		return nil
	}

	db := models.DB.WithContext(ctx)
	now := time.Now()
	date := at.Format("2006-01-02")
	hour := at.Hour()

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"tokens":     gorm.Expr("tokens + ?", tokens),
			"updated_at": now,
		}),
	}).Create(&models.StatsTotal{ID: 1, Tokens: tokens}).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"tokens":     gorm.Expr("tokens + ?", tokens),
			"updated_at": now,
		}),
	}).Create(&models.StatsDaily{Date: date, Tokens: tokens}).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}, {Name: "hour"}},
		DoUpdates: clause.Assignments(map[string]any{
			"tokens":     gorm.Expr("tokens + ?", tokens),
			"updated_at": now,
		}),
	}).Create(&models.StatsHourly{Date: date, Hour: hour, Tokens: tokens}).Error; err != nil {
		return err
	}

	return nil
}
