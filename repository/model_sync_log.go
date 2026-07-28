package repository

import (
	"context"
	"fmt"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ModelSyncLogFilter 模型同步日志筛选。
type ModelSyncLogFilter struct {
	ProviderID    string
	Status        string // 空表示按 ShowUnchanged 默认
	ShowUnchanged bool
}

// ModelSyncLogListOptions 列表选项。
type ModelSyncLogListOptions struct {
	Filter   ModelSyncLogFilter
	Page     int
	PageSize int
}

// ModelSyncLogListResult 分页结果。
type ModelSyncLogListResult struct {
	Logs  []models.ModelSyncLog
	Total int64
}

// ModelSyncLogRepo 封装 ModelSyncLog 数据访问。
type ModelSyncLogRepo interface {
	// List 分页列表；Status 非法时返回 error。
	List(ctx context.Context, opts ModelSyncLogListOptions) (*ModelSyncLogListResult, error)
	HardDeleteByIDs(ctx context.Context, ids []uint) (int64, error)
	HardDeleteAll(ctx context.Context) (int64, error)
	// HardDeleteErrors 硬删 status=error；providerIDs 非空时限定 provider。
	HardDeleteErrors(ctx context.Context, providerIDs []uint) (int64, error)
}

// NewModelSyncLogRepo 创建 ModelSyncLogRepo。
func NewModelSyncLogRepo(db *gorm.DB) ModelSyncLogRepo {
	return &modelSyncLogRepo{db: db}
}

type modelSyncLogRepo struct {
	db *gorm.DB
}

func (r *modelSyncLogRepo) List(ctx context.Context, opts ModelSyncLogListOptions) (*ModelSyncLogListResult, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := r.db.WithContext(ctx).Model(&models.ModelSyncLog{})
	if opts.Filter.ProviderID != "" {
		query = query.Where("provider_id = ?", opts.Filter.ProviderID)
	}

	if opts.Filter.Status != "" {
		switch opts.Filter.Status {
		case "success", "error", "unchanged":
			query = query.Where("status = ?", opts.Filter.Status)
		default:
			return nil, fmt.Errorf("invalid status: %s", opts.Filter.Status)
		}
	} else if !opts.Filter.ShowUnchanged {
		query = query.Where("status = ?", "success")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var logs []models.ModelSyncLog
	offset := (page - 1) * pageSize
	if err := query.Order("synced_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, err
	}

	return &ModelSyncLogListResult{Logs: logs, Total: total}, nil
}

func (r *modelSyncLogRepo) HardDeleteByIDs(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Unscoped().Where("id IN ?", ids).Delete(&models.ModelSyncLog{})
	return result.RowsAffected, result.Error
}

func (r *modelSyncLogRepo) HardDeleteAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&models.ModelSyncLog{})
	return result.RowsAffected, result.Error
}

func (r *modelSyncLogRepo) HardDeleteErrors(ctx context.Context, providerIDs []uint) (int64, error) {
	query := r.db.WithContext(ctx).Unscoped().Model(&models.ModelSyncLog{}).Where("status = ?", "error")
	if len(providerIDs) > 0 {
		query = query.Where("provider_id IN ?", providerIDs)
	}
	result := query.Delete(&models.ModelSyncLog{})
	return result.RowsAffected, result.Error
}
