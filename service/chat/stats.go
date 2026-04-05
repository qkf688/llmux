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

	if modelName == "" {
		return nil
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"calls":      gorm.Expr("calls + 1"),
			"updated_at": now,
		}),
	}).Create(&models.StatsModelTotal{Name: modelName, Calls: 1}).Error; err != nil {
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

	return nil
}

