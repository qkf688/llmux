package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// ErrApiKeyExists 表示新 key 的 hash 与既有行撞唯一索引。
// 查重语义走「DB 撞库翻译」而非应用层预查（多 api-key 方案 §5 定案：预检查询与
// 写入之间存在竞态窗口，uniqueIndex 才是单一真相），调用方据此映射 HTTP 409。
var ErrApiKeyExists = errors.New("repository: api key hash already exists")

// ApiKeyRepo 封装 /v1 调用 key（api_keys 表）的数据访问。
// 加解密不在本层：repo 存取的是已加密的存储行（与 CredentialRepo 同一边界，
// 密文契约见 models.ApiKey 注释）。
type ApiKeyRepo interface {
	// Create 落一行 key。hash 撞唯一索引 → ErrApiKeyExists；空 Key/KeyHash 属
	// 调用方装配错误，直接报错（SQLite NOT NULL 不拦空串，空 hash 行会永久占死
	// 唯一索引空槽位）。
	Create(ctx context.Context, key *models.ApiKey) error
	// FindByHash 按 SHA-256 hex 点查（/v1 鉴权入口，#4-2 接线）。未命中返回
	// gorm.ErrRecordNotFound，调用方区分「不存在」与「存在但 disabled」。
	FindByHash(ctx context.Context, hash string) (*models.ApiKey, error)
	// List 返回全部 key（管理端列表，含 disabled；密文原样返回，解密与降级归调用方）。
	List(ctx context.Context) ([]models.ApiKey, error)
	// Get 根据 ID 获取。
	Get(ctx context.Context, id uint) (*models.ApiKey, error)
	// UpdateFields 按字段 map 更新（重命名/启停；map 可显式写 false，绕过 struct
	// 零值跳过语义）。返回受影响行数。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// Delete 按 ID **硬删**（Unscoped）：api_keys 的 uniqueIndex 不含 deleted_at，
	// 软删残留会无声挡死「删后重建同值 key」——purgeSoftDeleted 同款教训。
	Delete(ctx context.Context, id uint) (int64, error)
}

// NewApiKeyRepo 创建 ApiKeyRepo 实现。
func NewApiKeyRepo(db *gorm.DB) ApiKeyRepo {
	return &apiKeyRepo{db: db}
}

type apiKeyRepo struct {
	db *gorm.DB
}

func (r *apiKeyRepo) Create(ctx context.Context, key *models.ApiKey) error {
	if key == nil || key.Key == "" || key.KeyHash == "" {
		return errors.New("repository: api key row requires non-empty ciphertext and hash")
	}
	err := r.db.WithContext(ctx).Create(key).Error
	if err == nil {
		return nil
	}
	// glebarez/sqlite 未开启 TranslateError，唯一冲突以驱动原文抛出；
	// 同时保留 ErrDuplicatedKey 分支，未来驱动行为变化不需回头改这里。
	if errors.Is(err, gorm.ErrDuplicatedKey) ||
		strings.Contains(err.Error(), "UNIQUE constraint failed: api_keys.api_key_hash") {
		return fmt.Errorf("%w: %w", err, ErrApiKeyExists)
	}
	return err
}

func (r *apiKeyRepo) FindByHash(ctx context.Context, hash string) (*models.ApiKey, error) {
	var key models.ApiKey
	if err := r.db.WithContext(ctx).Where("api_key_hash = ?", hash).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepo) List(ctx context.Context) ([]models.ApiKey, error) {
	var keys []models.ApiKey
	if err := r.db.WithContext(ctx).Order("id").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *apiKeyRepo) Get(ctx context.Context, id uint) (*models.ApiKey, error) {
	var key models.ApiKey
	if err := r.db.WithContext(ctx).First(&key, id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.ApiKey{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *apiKeyRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Delete(&models.ApiKey{}, id)
	return result.RowsAffected, result.Error
}
