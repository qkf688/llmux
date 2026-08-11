package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/qkf688/llmux/models"
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

// ModelSyncProviderStats 是「每个 provider 最新一次同步状态」的聚合结果。
// 注意口径：三个计数是 provider 数，不是日志行数——同一 provider 的历史行只算最新那条。
type ModelSyncProviderStats struct {
	ProvidersWithUpdates int        // 最新一次同步 status = success
	ProvidersUnchanged   int        // 最新一次同步 status = unchanged
	ProvidersWithErrors  int        // 最新一次同步 status = error
	ProvidersSynced      int        // 有同步记录的 provider 数
	LastSyncAt           *time.Time // 全局最近一次同步时间；无记录或零值时为 nil
}

// ModelSyncLogRepo 封装 ModelSyncLog 数据访问。
type ModelSyncLogRepo interface {
	// List 分页列表；Status 非法时返回 error。
	List(ctx context.Context, opts ModelSyncLogListOptions) (*ModelSyncLogListResult, error)
	// AggregateProviderStats 按 provider 取最新一次同步状态并分桶计数。
	// providerIDs 为空时返回零值统计（不退化为全表）。
	AggregateProviderStats(ctx context.Context, providerIDs []uint) (*ModelSyncProviderStats, error)
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

// normalizeSyncStatus 把日志行归一化到 success/error/unchanged 三态。
// 早期版本的 ModelSyncLog 没有 Status 字段，旧行 status 为空串；
// 兼容规则：空 status 时按增删数量反推（有增删 = success，否则 unchanged）。
// 该兜底只在此处存在——上层拿到的已是归一化状态，避免各调用方重放同一分支。
func normalizeSyncStatus(status string, addedCount, removedCount int) string {
	if status != "" {
		return status
	}
	if addedCount > 0 || removedCount > 0 {
		return "success"
	}
	return "unchanged"
}

func (r *modelSyncLogRepo) AggregateProviderStats(ctx context.Context, providerIDs []uint) (*ModelSyncProviderStats, error) {
	stats := &ModelSyncProviderStats{}
	// 空集合不能落到 SQL：GORM 的 `IN ()` 在部分驱动下语义不一致，
	// 且业务含义明确——没有可同步的 provider 就没有统计。
	if len(providerIDs) == 0 {
		return stats, nil
	}

	// 只取分桶所需的窄列，避免把 AddedModels/RemovedModels 两个 JSON blob 拉进内存。
	type statRow struct {
		ProviderID   uint
		Status       string
		AddedCount   int
		RemovedCount int
		SyncedAt     time.Time
	}
	var rows []statRow
	if err := r.db.WithContext(ctx).
		Model(&models.ModelSyncLog{}).
		Select("provider_id", "status", "added_count", "removed_count", "synced_at").
		Where("provider_id IN ?", providerIDs).
		Order("synced_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("aggregate model sync provider stats: %w", err)
	}

	// rows 已按 synced_at DESC 排序，每个 provider 首次出现即其最新一条。
	seen := make(map[uint]struct{}, len(providerIDs))
	for _, row := range rows {
		if _, exists := seen[row.ProviderID]; exists {
			continue
		}
		seen[row.ProviderID] = struct{}{}

		switch normalizeSyncStatus(row.Status, row.AddedCount, row.RemovedCount) {
		case "success":
			stats.ProvidersWithUpdates++
		case "unchanged":
			stats.ProvidersUnchanged++
		case "error":
			stats.ProvidersWithErrors++
		}

		if !row.SyncedAt.IsZero() && (stats.LastSyncAt == nil || row.SyncedAt.After(*stats.LastSyncAt)) {
			syncedAt := row.SyncedAt
			stats.LastSyncAt = &syncedAt
		}
	}
	stats.ProvidersSynced = len(seen)

	return stats, nil
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
