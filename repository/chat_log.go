package repository

import (
	"context"
	"strings"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ChatLogFilter 请求日志列表/条件清理筛选（对齐现网 query）。
type ChatLogFilter struct {
	ProviderName string
	Name         string
	Status       string
	Style        string
	UserAgent    string
}

// ChatLogListOptions 列表查询选项。
type ChatLogListOptions struct {
	Filter     ChatLogFilter
	IncludeRaw bool
	Page       int
	PageSize   int
}

// ChatLogListResult 分页列表结果。
type ChatLogListResult struct {
	Logs  []models.ChatLog
	Total int64
}

// ChatLogRepo 封装 ChatLog 数据访问。
type ChatLogRepo interface {
	// Get 根据 ID 获取日志（含大字段）。
	Get(ctx context.Context, id uint) (*models.ChatLog, error)
	// List 分页筛选列表。
	List(ctx context.Context, opts ChatLogListOptions) (*ChatLogListResult, error)
	// DistinctUserAgents 返回非空去重 user_agent。
	DistinctUserAgents(ctx context.Context) ([]string, error)
	// HardDelete 硬删单条日志，返回受影响行数。
	HardDelete(ctx context.Context, id uint) (int64, error)
	// HardDeleteByIDs 硬删多条日志。
	HardDeleteByIDs(ctx context.Context, ids []uint) (int64, error)
	// HardDeleteAll 硬删全部日志。
	HardDeleteAll(ctx context.Context) (int64, error)
	// HardDeleteFiltered 事务内：先硬删匹配筛选的 ChatIO，再硬删 ChatLog。
	HardDeleteFiltered(ctx context.Context, filter ChatLogFilter) (int64, error)
}

// ChatIORepo 封装 ChatIO 数据访问。
type ChatIORepo interface {
	// GetByLogID 按 log_id 获取。
	GetByLogID(ctx context.Context, logID uint) (*models.ChatIO, error)
	// HardDeleteByLogID 硬删指定 log 的 ChatIO。
	HardDeleteByLogID(ctx context.Context, logID uint) error
	// HardDeleteByLogIDs 硬删多条 log 的 ChatIO。
	HardDeleteByLogIDs(ctx context.Context, logIDs []uint) error
	// HardDeleteAll 硬删全部 ChatIO。
	HardDeleteAll(ctx context.Context) error
}

// NewChatLogRepo 创建 ChatLogRepo。
func NewChatLogRepo(db *gorm.DB) ChatLogRepo {
	return &chatLogRepo{db: db}
}

// NewChatIORepo 创建 ChatIORepo。
func NewChatIORepo(db *gorm.DB) ChatIORepo {
	return &chatIORepo{db: db}
}

type chatLogRepo struct {
	db *gorm.DB
}

type chatIORepo struct {
	db *gorm.DB
}

func applyChatLogFilter(query *gorm.DB, filter ChatLogFilter) *gorm.DB {
	providerName := strings.TrimSpace(filter.ProviderName)
	name := strings.TrimSpace(filter.Name)
	status := strings.TrimSpace(filter.Status)
	style := strings.TrimSpace(filter.Style)
	userAgent := strings.TrimSpace(filter.UserAgent)

	if providerName != "" {
		query = query.Where("provider_name = ?", providerName)
	}
	if name != "" {
		query = query.Where("name = ?", name)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if style != "" {
		query = query.Where("style = ?", style)
	}
	if userAgent != "" {
		query = query.Where("user_agent = ?", userAgent)
	}
	return query
}

func (r *chatLogRepo) Get(ctx context.Context, id uint) (*models.ChatLog, error) {
	var log models.ChatLog
	if err := r.db.WithContext(ctx).First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *chatLogRepo) List(ctx context.Context, opts ChatLogListOptions) (*ChatLogListResult, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	baseQuery := applyChatLogFilter(r.db.WithContext(ctx).Model(&models.ChatLog{}), opts.Filter)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	query := baseQuery
	if !opts.IncludeRaw {
		query = query.Select(
			"id",
			"created_at",
			"name",
			"provider_model",
			"provider_name",
			"status",
			"style",
			"user_agent",
			"remote_ip",
			"chat_io",
			"error",
			"retry",
			"proxy_time",
			"first_chunk_time",
			"chunk_time",
			"tps",
			"prompt_tokens",
			"completion_tokens",
			"total_tokens",
			"prompt_tokens_details",
			"completion_tokens_details",
		)
	}

	var logs []models.ChatLog
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, err
	}

	return &ChatLogListResult{Logs: logs, Total: total}, nil
}

func (r *chatLogRepo) DistinctUserAgents(ctx context.Context) ([]string, error) {
	var userAgents []string
	err := r.db.WithContext(ctx).Model(&models.ChatLog{}).
		Where("user_agent IS NOT NULL AND user_agent != ''").
		Distinct("user_agent").
		Pluck("user_agent", &userAgents).Error
	return userAgents, err
}

func (r *chatLogRepo) HardDelete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&models.ChatLog{})
	return result.RowsAffected, result.Error
}

func (r *chatLogRepo) HardDeleteByIDs(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Unscoped().Where("id IN ?", ids).Delete(&models.ChatLog{})
	return result.RowsAffected, result.Error
}

func (r *chatLogRepo) HardDeleteAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&models.ChatLog{})
	return result.RowsAffected, result.Error
}

func (r *chatLogRepo) HardDeleteFiltered(ctx context.Context, filter ChatLogFilter) (int64, error) {
	var deleted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subQuery := applyChatLogFilter(tx.Model(&models.ChatLog{}).Select("id"), filter)

		if err := tx.Unscoped().
			Where("log_id IN (?)", subQuery).
			Delete(&models.ChatIO{}).Error; err != nil {
			return err
		}

		result := tx.Unscoped().
			Where("id IN (?)", subQuery).
			Delete(&models.ChatLog{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected
		return nil
	})
	return deleted, err
}

func (r *chatIORepo) GetByLogID(ctx context.Context, logID uint) (*models.ChatIO, error) {
	var row models.ChatIO
	if err := r.db.WithContext(ctx).Where("log_id = ?", logID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *chatIORepo) HardDeleteByLogID(ctx context.Context, logID uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("log_id = ?", logID).Delete(&models.ChatIO{}).Error
}

func (r *chatIORepo) HardDeleteByLogIDs(ctx context.Context, logIDs []uint) error {
	if len(logIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Unscoped().Where("log_id IN ?", logIDs).Delete(&models.ChatIO{}).Error
}

func (r *chatIORepo) HardDeleteAll(ctx context.Context) error {
	return r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&models.ChatIO{}).Error
}