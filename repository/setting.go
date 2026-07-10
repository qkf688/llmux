package repository

import (
	"context"
	"strconv"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// SettingRepo 封装 Setting 键值对的数据访问。
type SettingRepo interface {
	// Get 根据 key 获取 Setting 记录。
	Get(ctx context.Context, key string) (*models.Setting, error)
	// GetValue 根据 key 获取 value 字符串；记录不存在时返回空字符串。
	GetValue(ctx context.Context, key string) (string, error)
	// Set 设置 key 的值。
	Set(ctx context.Context, key, value string) error
	// SetBool 设置布尔值。
	SetBool(ctx context.Context, key string, value bool) error
	// SetInt 设置整数值。
	SetInt(ctx context.Context, key string, value int) error
	// List 返回所有 Setting 记录。
	List(ctx context.Context) ([]models.Setting, error)
	// Exists 判断 key 是否存在。
	Exists(ctx context.Context, key string) (bool, error)
}

// NewSettingRepo 创建 SettingRepo 实现。
func NewSettingRepo(db *gorm.DB) SettingRepo {
	return &settingRepo{db: db}
}

type settingRepo struct {
	db *gorm.DB
}

func (r *settingRepo) Get(ctx context.Context, key string) (*models.Setting, error) {
	var setting models.Setting
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *settingRepo) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return setting.Value, nil
}

func (r *settingRepo) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).
		Where("key = ?", key).
		Assign(&models.Setting{Key: key, Value: value}).
		FirstOrCreate(&models.Setting{}).Error
}

func (r *settingRepo) SetBool(ctx context.Context, key string, value bool) error {
	strValue := "false"
	if value {
		strValue = "true"
	}
	return r.Set(ctx, key, strValue)
}

func (r *settingRepo) SetInt(ctx context.Context, key string, value int) error {
	return r.Set(ctx, key, strconv.Itoa(value))
}

func (r *settingRepo) List(ctx context.Context) ([]models.Setting, error) {
	var settings []models.Setting
	if err := r.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *settingRepo) Exists(ctx context.Context, key string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
