package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ModelWithProviderRepo 封装模型-供应商关联的数据访问（接口形状对齐现网 handler 查询）。
type ModelWithProviderRepo interface {
	// ListByModelID 返回指定模型的全部关联。
	ListByModelID(ctx context.Context, modelID uint) ([]models.ModelWithProvider, error)
	// ListAll 返回全部关联。
	ListAll(ctx context.Context) ([]models.ModelWithProvider, error)
	// Get 根据 ID 获取关联。
	Get(ctx context.Context, id uint) (*models.ModelWithProvider, error)
	// Create 创建关联。
	Create(ctx context.Context, assoc *models.ModelWithProvider) error
	// Update 根据 ID 更新关联字段（GORM Updates：结构体零值字段不写入）。
	Update(ctx context.Context, id uint, updates models.ModelWithProvider) error
	// Delete 根据 ID 删除关联，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// DeleteByIDs 批量删除关联，返回受影响行数。
	DeleteByIDs(ctx context.Context, ids []uint) (int64, error)
	// UpdateByIDs 批量更新关联字段，返回受影响行数。
	UpdateByIDs(ctx context.Context, ids []uint, updates models.ModelWithProvider) (int64, error)
	// DeleteByProviderID 删除指定供应商下的全部关联，返回受影响行数。
	DeleteByProviderID(ctx context.Context, providerID uint) (int64, error)
}

// NewModelWithProviderRepo 创建 ModelWithProviderRepo 实现。
func NewModelWithProviderRepo(db *gorm.DB) ModelWithProviderRepo {
	return &modelWithProviderRepo{db: db}
}

type modelWithProviderRepo struct {
	db *gorm.DB
}

func (r *modelWithProviderRepo) ListByModelID(ctx context.Context, modelID uint) ([]models.ModelWithProvider, error) {
	var rows []models.ModelWithProvider
	if err := r.db.WithContext(ctx).Where("model_id = ?", modelID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *modelWithProviderRepo) ListAll(ctx context.Context) ([]models.ModelWithProvider, error) {
	var rows []models.ModelWithProvider
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *modelWithProviderRepo) Get(ctx context.Context, id uint) (*models.ModelWithProvider, error) {
	var row models.ModelWithProvider
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *modelWithProviderRepo) Create(ctx context.Context, assoc *models.ModelWithProvider) error {
	return r.db.WithContext(ctx).Create(assoc).Error
}

func (r *modelWithProviderRepo) Update(ctx context.Context, id uint, updates models.ModelWithProvider) error {
	return r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).Where("id = ?", id).Updates(updates).Error
}

func (r *modelWithProviderRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DeleteByIDs(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) UpdateByIDs(ctx context.Context, ids []uint, updates models.ModelWithProvider) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).Where("id IN ?", ids).Updates(updates)
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DeleteByProviderID(ctx context.Context, providerID uint) (int64, error) {
	result := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}