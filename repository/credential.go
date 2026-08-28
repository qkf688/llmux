package repository

import (
	"context"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// CredentialRepo 封装凭据（credentials）实体的数据访问。
// 凭据 Key 在 models 层为密文（hex(nonce‖ciphertext)），本层原样存取不解密——
// 解密/掩码是上层（S2 API / S3 调度）的职责。
type CredentialRepo interface {
	// List 返回符合条件的凭据；filter 为零值时返回全部。
	List(ctx context.Context, filter CredentialFilter) ([]models.Credential, error)
	// Get 根据 ID 获取凭据。
	Get(ctx context.Context, id uint) (*models.Credential, error)
	// Create 创建凭据。
	Create(ctx context.Context, cred *models.Credential) error
	// Update 根据 ID 更新凭据。struct 更新会跳过零值字段——清空三态指针
	// （如切换凭据来源置空 PoolID/GroupID）必须用 UpdateFields。
	Update(ctx context.Context, id uint, cred *models.Credential) error
	// UpdateFields 按字段 map 更新（map 可显式写 NULL，绕过 struct 零值跳过语义）。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// Delete 根据 ID 删除凭据，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
}

// CredentialFilter 用于凭据 List 查询的筛选条件。
type CredentialFilter struct {
	PoolID  *uint
	GroupID *uint
	Status  string
	KeyHash string
}

// NewCredentialRepo 创建 CredentialRepo 实现。
func NewCredentialRepo(db *gorm.DB) CredentialRepo {
	return &credentialRepo{db: db}
}

type credentialRepo struct {
	db *gorm.DB
}

func (r *credentialRepo) List(ctx context.Context, filter CredentialFilter) ([]models.Credential, error) {
	query := r.db.WithContext(ctx).Model(&models.Credential{})
	if filter.PoolID != nil {
		query = query.Where("pool_id = ?", *filter.PoolID)
	}
	if filter.GroupID != nil {
		query = query.Where("group_id = ?", *filter.GroupID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.KeyHash != "" {
		query = query.Where("key_hash = ?", filter.KeyHash)
	}
	var creds []models.Credential
	if err := query.Find(&creds).Error; err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *credentialRepo) Get(ctx context.Context, id uint) (*models.Credential, error) {
	var cred models.Credential
	if err := r.db.WithContext(ctx).First(&cred, id).Error; err != nil {
		return nil, err
	}
	return &cred, nil
}

func (r *credentialRepo) Create(ctx context.Context, cred *models.Credential) error {
	return r.db.WithContext(ctx).Create(cred).Error
}

func (r *credentialRepo) Update(ctx context.Context, id uint, cred *models.Credential) error {
	return r.db.WithContext(ctx).Model(&models.Credential{}).Where("id = ?", id).Updates(cred).Error
}

func (r *credentialRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.Credential{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *credentialRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.Credential{}, id)
	return result.RowsAffected, result.Error
}
