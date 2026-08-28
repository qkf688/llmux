package repository

import (
	"context"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// PoolRepo 封装号池（pools）实体的数据访问。
type PoolRepo interface {
	// List 返回符合条件的号池；filter 为零值时返回全部。
	List(ctx context.Context, filter PoolFilter) ([]models.Pool, error)
	// Get 根据 ID 获取号池。
	Get(ctx context.Context, id uint) (*models.Pool, error)
	// Create 创建号池。
	Create(ctx context.Context, pool *models.Pool) error
	// Update 根据 ID 更新号池。
	Update(ctx context.Context, id uint, pool *models.Pool) error
	// Delete 根据 ID 删除号池，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
}

// PoolFilter 用于号池 List 查询的筛选条件。
type PoolFilter struct {
	Name string
}

// NewPoolRepo 创建 PoolRepo 实现。
func NewPoolRepo(db *gorm.DB) PoolRepo {
	return &poolRepo{db: db}
}

type poolRepo struct {
	db *gorm.DB
}

func (r *poolRepo) List(ctx context.Context, filter PoolFilter) ([]models.Pool, error) {
	query := r.db.WithContext(ctx).Model(&models.Pool{})
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	var pools []models.Pool
	if err := query.Find(&pools).Error; err != nil {
		return nil, err
	}
	return pools, nil
}

func (r *poolRepo) Get(ctx context.Context, id uint) (*models.Pool, error) {
	var pool models.Pool
	if err := r.db.WithContext(ctx).First(&pool, id).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

func (r *poolRepo) Create(ctx context.Context, pool *models.Pool) error {
	return r.db.WithContext(ctx).Create(pool).Error
}

func (r *poolRepo) Update(ctx context.Context, id uint, pool *models.Pool) error {
	return r.db.WithContext(ctx).Model(&models.Pool{}).Where("id = ?", id).Updates(pool).Error
}

func (r *poolRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.Pool{}, id)
	return result.RowsAffected, result.Error
}
