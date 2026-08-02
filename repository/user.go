package repository

import (
	"context"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// UserRepo 封装 User 的数据访问（鉴权专用）。
type UserRepo interface {
	// FindByUsername 按用户名查用户。
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	// FindByAPIKey 按 API key 查用户（/v1 代理鉴权用）。
	FindByAPIKey(ctx context.Context, key string) (*models.User, error)
	// GetByID 按 ID 查用户。
	GetByID(ctx context.Context, id uint) (*models.User, error)
	// Create 创建用户。
	Create(ctx context.Context, user *models.User) error
	// Count 返回用户总数（bootstrap 判断用）。
	Count(ctx context.Context) (int64, error)
	// UpdateAPIKey 更新用户的 API key。
	UpdateAPIKey(ctx context.Context, id uint, key string) error
	// UpdatePassword 更新用户的密码哈希。
	UpdatePassword(ctx context.Context, id uint, hash string) error
}

// NewUserRepo 创建 UserRepo 实现。
func NewUserRepo(db *gorm.DB) UserRepo {
	return &userRepo{db: db}
}

type userRepo struct {
	db *gorm.DB
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) FindByAPIKey(ctx context.Context, key string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("api_key = ?", key).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) GetByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userRepo) UpdateAPIKey(ctx context.Context, id uint, key string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("api_key", key).Error
}

func (r *userRepo) UpdatePassword(ctx context.Context, id uint, hash string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}
