package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// HealthCheckLogFilter 健康检查日志筛选。
type HealthCheckLogFilter struct {
	ModelProviderID string
	ModelName       string
	ProviderName    string
	Status          string
}

// HealthCheckLogListOptions 列表选项。
type HealthCheckLogListOptions struct {
	Filter   HealthCheckLogFilter
	Page     int
	PageSize int
}

// HealthCheckLogListResult 分页结果。
type HealthCheckLogListResult struct {
	Logs  []models.HealthCheckLog
	Total int64
}

// HealthCheckLogRepo 封装 HealthCheckLog 数据访问。
type HealthCheckLogRepo interface {
	// Create 创建健康检测日志。
	Create(ctx context.Context, log *models.HealthCheckLog) error
	// ListRecentByModelProviderID 按 checked_at 倒序返回指定关联最近 limit 条日志。
	ListRecentByModelProviderID(ctx context.Context, modelProviderID uint, limit int) ([]models.HealthCheckLog, error)
	List(ctx context.Context, opts HealthCheckLogListOptions) (*HealthCheckLogListResult, error)
	HardDeleteAll(ctx context.Context) (int64, error)
	// EnforceRetention 保留最新 retention 条，超出部分软删（禁止硬删）。
	// retention<=0 时不清理。返回实际删除的条数。
	EnforceRetention(ctx context.Context, retention int) (int, error)
}

// NewHealthCheckLogRepo 创建 HealthCheckLogRepo。
func NewHealthCheckLogRepo(db *gorm.DB) HealthCheckLogRepo {
	return &healthCheckLogRepo{db: db}
}

type healthCheckLogRepo struct {
	db *gorm.DB
}

func (r *healthCheckLogRepo) Create(ctx context.Context, log *models.HealthCheckLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *healthCheckLogRepo) ListRecentByModelProviderID(ctx context.Context, modelProviderID uint, limit int) ([]models.HealthCheckLog, error) {
	var logs []models.HealthCheckLog
	if err := r.db.WithContext(ctx).
		Where("model_provider_id = ?", modelProviderID).
		Order("checked_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *healthCheckLogRepo) List(ctx context.Context, opts HealthCheckLogListOptions) (*HealthCheckLogListResult, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	query := r.db.WithContext(ctx).Model(&models.HealthCheckLog{})
	if opts.Filter.ModelProviderID != "" {
		query = query.Where("model_provider_id = ?", opts.Filter.ModelProviderID)
	}
	if opts.Filter.ModelName != "" {
		query = query.Where("model_name = ?", opts.Filter.ModelName)
	}
	if opts.Filter.ProviderName != "" {
		query = query.Where("provider_name = ?", opts.Filter.ProviderName)
	}
	if opts.Filter.Status != "" {
		query = query.Where("status = ?", opts.Filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var logs []models.HealthCheckLog
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, err
	}

	return &HealthCheckLogListResult{Logs: logs, Total: total}, nil
}

func (r *healthCheckLogRepo) HardDeleteAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&models.HealthCheckLog{})
	return result.RowsAffected, result.Error
}

func (r *healthCheckLogRepo) EnforceRetention(ctx context.Context, retention int) (int, error) {
	// HealthCheck：普通 Count + 软删（禁止 Unscoped 硬删）
	return models.EnforceRetentionByOldestID(ctx, r.db, &models.HealthCheckLog{}, retention, models.RetentionDeleteOptions{
		UnscopedCount:  false,
		UnscopedDelete: false,
	})
}