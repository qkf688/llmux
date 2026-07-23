package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ModelTemplateItemRepo 封装模型模板项的数据访问（接口形状对齐现网 handler 查询）。
type ModelTemplateItemRepo interface {
	// CountByModelIDAndName 统计指定模型下同名模板项数量（区分大小写）。
	CountByModelIDAndName(ctx context.Context, modelID uint, name string) (int64, error)
	// Create 创建模板项。
	Create(ctx context.Context, item *models.ModelTemplateItem) error
	// ListAll 返回全部模板项。
	ListAll(ctx context.Context) ([]models.ModelTemplateItem, error)
	// ListByModelID 返回指定模型的全部模板项。
	ListByModelID(ctx context.Context, modelID uint) ([]models.ModelTemplateItem, error)
	// DeleteByModelIDAndNameUnscoped 硬删指定模型下的同名模板项。
	DeleteByModelIDAndNameUnscoped(ctx context.Context, modelID uint, name string) (int64, error)
}

// NewModelTemplateItemRepo 创建 ModelTemplateItemRepo 实现。
func NewModelTemplateItemRepo(db *gorm.DB) ModelTemplateItemRepo {
	return &modelTemplateItemRepo{db: db}
}

type modelTemplateItemRepo struct {
	db *gorm.DB
}

func (r *modelTemplateItemRepo) CountByModelIDAndName(ctx context.Context, modelID uint, name string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ModelTemplateItem{}).
		Where("model_id = ? AND name = ?", modelID, name).
		Count(&count).Error
	return count, err
}

func (r *modelTemplateItemRepo) Create(ctx context.Context, item *models.ModelTemplateItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *modelTemplateItemRepo) ListAll(ctx context.Context) ([]models.ModelTemplateItem, error) {
	var items []models.ModelTemplateItem
	if err := r.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *modelTemplateItemRepo) ListByModelID(ctx context.Context, modelID uint) ([]models.ModelTemplateItem, error) {
	var items []models.ModelTemplateItem
	if err := r.db.WithContext(ctx).Where("model_id = ?", modelID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *modelTemplateItemRepo) DeleteByModelIDAndNameUnscoped(ctx context.Context, modelID uint, name string) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().
		Where("model_id = ? AND name = ?", modelID, name).
		Delete(&models.ModelTemplateItem{})
	return result.RowsAffected, result.Error
}
