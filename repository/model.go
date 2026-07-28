package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ModelRepo 封装 Model 实体的数据访问。
type ModelRepo interface {
	// List 返回所有真实模型。
	List(ctx context.Context) ([]models.Model, error)
	// Get 根据 ID 获取模型。
	Get(ctx context.Context, id uint) (*models.Model, error)
	// GetByName 根据名称获取模型。
	GetByName(ctx context.Context, name string) (*models.Model, error)
	// ExistsByName 判断指定名称的模型是否存在。
	ExistsByName(ctx context.Context, name string) (bool, error)
	// Create 创建模型。
	Create(ctx context.Context, model *models.Model) error
	// Update 根据 ID 更新模型（GORM Updates：结构体零值字段不写入）。
	Update(ctx context.Context, id uint, model *models.Model) error
	// UpdateFields 按列名更新指定模型（可写入零值），返回受影响行数。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// Delete 根据 ID 删除模型。
	Delete(ctx context.Context, id uint) (int64, error)
	// DeleteByIDs 批量删除模型，返回受影响行数。
	DeleteByIDs(ctx context.Context, ids []uint) (int64, error)
	// ListByIDs 返回指定 ID 集合中的模型。
	ListByIDs(ctx context.Context, ids []uint) ([]models.Model, error)
	// BatchUpdate 批量更新指定 ID 的模型字段。
	BatchUpdate(ctx context.Context, ids []uint, updates map[string]any) (int64, error)
}

// NewModelRepo 创建 ModelRepo 实现。
func NewModelRepo(db *gorm.DB) ModelRepo {
	return &modelRepo{db: db}
}

type modelRepo struct {
	db *gorm.DB
}

func (r *modelRepo) List(ctx context.Context) ([]models.Model, error) {
	var modelsList []models.Model
	if err := r.db.WithContext(ctx).Find(&modelsList).Error; err != nil {
		return nil, err
	}
	return modelsList, nil
}

func (r *modelRepo) Get(ctx context.Context, id uint) (*models.Model, error) {
	var model models.Model
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

func (r *modelRepo) GetByName(ctx context.Context, name string) (*models.Model, error) {
	var model models.Model
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

func (r *modelRepo) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Model{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *modelRepo) Create(ctx context.Context, model *models.Model) error {
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *modelRepo) Update(ctx context.Context, id uint, model *models.Model) error {
	return r.db.WithContext(ctx).Model(&models.Model{}).Where("id = ?", id).Updates(model).Error
}

func (r *modelRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.Model{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *modelRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.Model{}, id)
	return result.RowsAffected, result.Error
}

func (r *modelRepo) DeleteByIDs(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&models.Model{})
	return result.RowsAffected, result.Error
}

func (r *modelRepo) ListByIDs(ctx context.Context, ids []uint) ([]models.Model, error) {
	if len(ids) == 0 {
		return []models.Model{}, nil
	}
	var modelsList []models.Model
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&modelsList).Error; err != nil {
		return nil, err
	}
	return modelsList, nil
}

func (r *modelRepo) BatchUpdate(ctx context.Context, ids []uint, updates map[string]any) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.Model{}).Where("id IN ?", ids).Updates(updates)
	return result.RowsAffected, result.Error
}
