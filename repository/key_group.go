package repository

import (
	"context"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// KeyGroupRepo 封装凭据分组（key_groups）实体的数据访问。
type KeyGroupRepo interface {
	// ListByProvider 返回指定供应商的全部分组。
	ListByProvider(ctx context.Context, providerID uint) ([]models.KeyGroup, error)
	// Get 根据 ID 获取分组。
	Get(ctx context.Context, id uint) (*models.KeyGroup, error)
	// Create 创建分组。
	Create(ctx context.Context, group *models.KeyGroup) error
	// Update 根据 ID 更新分组。struct 更新会跳过零值字段——清空三态指针
	// （如切换凭据来源置空 PoolID）必须用 UpdateFields。
	Update(ctx context.Context, id uint, group *models.KeyGroup) error
	// UpdateFields 按字段 map 更新（map 可显式写 NULL，绕过 struct 零值跳过语义）。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// Delete 根据 ID 删除分组，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// CountByPoolIDs 返回每个号池被分组引用的次数（key_groups.PoolID 指向该号池）。
	// 号池删除守卫用：被引用（>0）时禁止删除。无引用的号池不产生条目。
	CountByPoolIDs(ctx context.Context, poolIDs []uint) (map[uint]int64, error)
	// CountByProviderIDs 返回每个供应商的分组数量（供应商列表徽标用）。
	// 无分组的供应商不产生条目（调用方对缺失键按 0 处理）。
	CountByProviderIDs(ctx context.Context, providerIDs []uint) (map[uint]int64, error)
}

// NewKeyGroupRepo 创建 KeyGroupRepo 实现。
func NewKeyGroupRepo(db *gorm.DB) KeyGroupRepo {
	return &keyGroupRepo{db: db}
}

type keyGroupRepo struct {
	db *gorm.DB
}

func (r *keyGroupRepo) ListByProvider(ctx context.Context, providerID uint) ([]models.KeyGroup, error) {
	var groups []models.KeyGroup
	if err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *keyGroupRepo) Get(ctx context.Context, id uint) (*models.KeyGroup, error) {
	var group models.KeyGroup
	if err := r.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *keyGroupRepo) Create(ctx context.Context, group *models.KeyGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *keyGroupRepo) Update(ctx context.Context, id uint, group *models.KeyGroup) error {
	return r.db.WithContext(ctx).Model(&models.KeyGroup{}).Where("id = ?", id).Updates(group).Error
}

func (r *keyGroupRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.KeyGroup{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *keyGroupRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.KeyGroup{}, id)
	return result.RowsAffected, result.Error
}

func (r *keyGroupRepo) CountByPoolIDs(ctx context.Context, poolIDs []uint) (map[uint]int64, error) {
	if len(poolIDs) == 0 {
		return map[uint]int64{}, nil
	}

	type countRow struct {
		PoolID uint
		Count  int64
	}
	var rows []countRow
	if err := r.db.WithContext(ctx).
		Model(&models.KeyGroup{}).
		Select("pool_id, COUNT(*) AS count").
		Where("pool_id IN ?", poolIDs).
		Group("pool_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.PoolID] = row.Count
	}
	return counts, nil
}

func (r *keyGroupRepo) CountByProviderIDs(ctx context.Context, providerIDs []uint) (map[uint]int64, error) {
	if len(providerIDs) == 0 {
		return map[uint]int64{}, nil
	}

	type countRow struct {
		ProviderID uint
		Count      int64
	}
	var rows []countRow
	if err := r.db.WithContext(ctx).
		Model(&models.KeyGroup{}).
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
