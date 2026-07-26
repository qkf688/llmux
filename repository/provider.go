package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ProviderRepo 封装 Provider 实体的数据访问。
type ProviderRepo interface {
	// List 返回符合条件的 Provider 列表；filter 为零值时返回全部。
	List(ctx context.Context, filter ProviderFilter) ([]models.Provider, error)
	// ListByIDs 返回指定 ID 集合中的 Provider。
	ListByIDs(ctx context.Context, ids []uint) ([]models.Provider, error)
	// Get 根据 ID 获取 Provider。
	Get(ctx context.Context, id uint) (*models.Provider, error)
	// GetByName 根据名称获取 Provider。
	GetByName(ctx context.Context, name string) (*models.Provider, error)
	// ExistsByName 判断指定名称的 Provider 是否存在。
	ExistsByName(ctx context.Context, name string) (bool, error)
	// Create 创建 Provider。
	Create(ctx context.Context, provider *models.Provider) error
	// Update 根据 ID 更新 Provider。
	Update(ctx context.Context, id uint, provider *models.Provider) error
	// Delete 根据 ID 删除 Provider。
	Delete(ctx context.Context, id uint) (int64, error)
	// UpdateBlacklist 整体替换黑名单状态，返回受影响的行数。
	UpdateBlacklist(ctx context.Context, ids []uint) (int64, error)
}

// ProviderFilter 用于 List 查询的筛选条件。
type ProviderFilter struct {
	Name          string
	Type          string
	Blacklisted   *bool
	ModelEndpoint *bool
}

// NewProviderRepo 创建 ProviderRepo 实现。
func NewProviderRepo(db *gorm.DB) ProviderRepo {
	return &providerRepo{db: db}
}

type providerRepo struct {
	db *gorm.DB
}

func (r *providerRepo) List(ctx context.Context, filter ProviderFilter) ([]models.Provider, error) {
	query := r.db.WithContext(ctx).Model(&models.Provider{})
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Blacklisted != nil {
		query = query.Where("blacklisted = ?", *filter.Blacklisted)
	}
	if filter.ModelEndpoint != nil {
		query = query.Where("model_endpoint = ?", *filter.ModelEndpoint)
	}

	var providers []models.Provider
	if err := query.Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

func (r *providerRepo) ListByIDs(ctx context.Context, ids []uint) ([]models.Provider, error) {
	if len(ids) == 0 {
		return []models.Provider{}, nil
	}
	var providers []models.Provider
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

func (r *providerRepo) Get(ctx context.Context, id uint) (*models.Provider, error) {
	var provider models.Provider
	if err := r.db.WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *providerRepo) GetByName(ctx context.Context, name string) (*models.Provider, error) {
	var provider models.Provider
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&provider).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *providerRepo) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Provider{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *providerRepo) Create(ctx context.Context, provider *models.Provider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

func (r *providerRepo) Update(ctx context.Context, id uint, provider *models.Provider) error {
	return r.db.WithContext(ctx).Model(&models.Provider{}).Where("id = ?", id).Updates(provider).Error
}

func (r *providerRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.Provider{}, id)
	return result.RowsAffected, result.Error
}

func (r *providerRepo) UpdateBlacklist(ctx context.Context, ids []uint) (int64, error) {
	var rowsAffected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		falseVal := false
		if err := tx.Model(&models.Provider{}).Where("blacklisted = ?", true).Update("blacklisted", &falseVal).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			trueVal := true
			result := tx.Model(&models.Provider{}).Where("id IN ?", ids).Update("blacklisted", &trueVal)
			if result.Error != nil {
				return result.Error
			}
			rowsAffected = result.RowsAffected
		}
		return nil
	})
	return rowsAffected, err
}
