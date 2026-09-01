package repository

import (
	"context"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// EndpointRepo 封装协议端点（endpoints）实体的数据访问。
type EndpointRepo interface {
	// ListByProvider 返回指定供应商的全部端点。
	ListByProvider(ctx context.Context, providerID uint) ([]models.Endpoint, error)
	// Get 根据 ID 获取端点。
	Get(ctx context.Context, id uint) (*models.Endpoint, error)
	// Create 创建端点。
	Create(ctx context.Context, endpoint *models.Endpoint) error
	// Update 根据 ID 更新端点。struct 更新会跳过零值——清空 URL / Enabled=false
	// 必须用 UpdateFields。
	Update(ctx context.Context, id uint, endpoint *models.Endpoint) error
	// UpdateFields 按字段 map 更新（map 可显式写空串/false）。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// Delete 根据 ID 删除端点，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// CountByProviderIDs 返回每个供应商的端点数量（供应商列表徽标用）。
	// 无端点的供应商不产生条目（调用方对缺失键按 0 处理）。
	CountByProviderIDs(ctx context.Context, providerIDs []uint) (map[uint]int64, error)
}

// NewEndpointRepo 创建 EndpointRepo 实现。
func NewEndpointRepo(db *gorm.DB) EndpointRepo {
	return &endpointRepo{db: db}
}

type endpointRepo struct {
	db *gorm.DB
}

func (r *endpointRepo) ListByProvider(ctx context.Context, providerID uint) ([]models.Endpoint, error) {
	var endpoints []models.Endpoint
	if err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Find(&endpoints).Error; err != nil {
		return nil, err
	}
	return endpoints, nil
}

func (r *endpointRepo) Get(ctx context.Context, id uint) (*models.Endpoint, error) {
	var endpoint models.Endpoint
	if err := r.db.WithContext(ctx).First(&endpoint, id).Error; err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func (r *endpointRepo) Create(ctx context.Context, endpoint *models.Endpoint) error {
	return r.db.WithContext(ctx).Create(endpoint).Error
}

func (r *endpointRepo) Update(ctx context.Context, id uint, endpoint *models.Endpoint) error {
	return r.db.WithContext(ctx).Model(&models.Endpoint{}).Where("id = ?", id).Updates(endpoint).Error
}

func (r *endpointRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.Endpoint{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *endpointRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.Endpoint{}, id)
	return result.RowsAffected, result.Error
}

func (r *endpointRepo) CountByProviderIDs(ctx context.Context, providerIDs []uint) (map[uint]int64, error) {
	if len(providerIDs) == 0 {
		return map[uint]int64{}, nil
	}

	type countRow struct {
		ProviderID uint
		Count      int64
	}
	var rows []countRow
	if err := r.db.WithContext(ctx).
		Model(&models.Endpoint{}).
		Select("provider_id, COUNT(*) AS count").
		Where("provider_id IN ?", providerIDs).
		Group("provider_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.ProviderID] = row.Count
	}
	return counts, nil
}
