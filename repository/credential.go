package repository

import (
	"context"
	"strings"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// CredentialRepo 封装凭据（credentials）实体的数据访问。
// 凭据 Key 在 models 层为密文（hex(nonce‖ciphertext)），本层原样存取不解密——
// 解密/掩码是上层（S2 API / S3 调度）的职责。
type CredentialRepo interface {
	// List 返回符合条件的凭据；filter 为零值时返回全部。
	List(ctx context.Context, filter CredentialFilter) ([]models.Credential, error)
	// ListPaged 返回分页凭据列表及总数；page/pageSize 由调用方校验（>=1），
	// Q 仅对 Note 做 LIKE 匹配（Key 需解密，不在此层搜索）。
	ListPaged(ctx context.Context, filter CredentialFilter, page, pageSize int) ([]models.Credential, int64, error)
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
	// DeleteByPoolID 删除指定号池下的全部凭据（软删），返回受影响行数。
	// 号池删除时的级联清理用；分组内联凭据（GroupID 归属）不受影响。
	DeleteByPoolID(ctx context.Context, poolID uint) (int64, error)
	// UpdateStatusByIDs 批量更新同一号池下指定 IDs 的状态（池内限界防越池）。
	UpdateStatusByIDs(ctx context.Context, poolID uint, ids []uint, status string) (int64, error)
	// DeleteByIDs 批量软删同一号池下指定 IDs 的凭据（池内限界防越池）。
	DeleteByIDs(ctx context.Context, poolID uint, ids []uint) (int64, error)
	// ExistingHashes 返回指定号池下现存（未软删）凭据中命中的 KeyHash 集合，
	// 供批量导入一次查询完成池内查重（防逐条 N+1）。KeyHash 非 unique，去重是
	// 应用层语义；跨池同 key 复用合法，不在此处做全局拦截。
	ExistingHashes(ctx context.Context, poolID uint, hashes []string) (map[string]bool, error)
}

// CredentialFilter 用于凭据 List 查询的筛选条件。
type CredentialFilter struct {
	PoolID  *uint
	GroupID *uint
	Status  string
	KeyHash string
	// Q 关键词搜索：仅对 Note 做 LIKE %Q%（Key 明文需解密，不在此层搜索）。
	Q string
}

// NewCredentialRepo 创建 CredentialRepo 实现。
func NewCredentialRepo(db *gorm.DB) CredentialRepo {
	return &credentialRepo{db: db}
}

type credentialRepo struct {
	db *gorm.DB
}

func applyCredentialFilter(query *gorm.DB, filter CredentialFilter) *gorm.DB {
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
	if filter.Q != "" {
		// 转义 LIKE 通配符，避免 q=% 匹配全表（AC-3 精确子串语义）
		escaped := strings.ReplaceAll(filter.Q, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `%`, `\%`)
		escaped = strings.ReplaceAll(escaped, `_`, `\_`)
		query = query.Where("note LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}
	return query
}

func (r *credentialRepo) List(ctx context.Context, filter CredentialFilter) ([]models.Credential, error) {
	query := applyCredentialFilter(r.db.WithContext(ctx).Model(&models.Credential{}), filter)
	var creds []models.Credential
	if err := query.Find(&creds).Error; err != nil {
		return nil, err
	}
	return creds, nil
}

// ListPaged 返回分页凭据列表及总数，按 id ASC 稳定排序。
func (r *credentialRepo) ListPaged(ctx context.Context, filter CredentialFilter, page, pageSize int) ([]models.Credential, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	base := applyCredentialFilter(r.db.WithContext(ctx).Model(&models.Credential{}), filter)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var creds []models.Credential
	if err := base.Order("id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&creds).Error; err != nil {
		return nil, 0, err
	}
	return creds, total, nil
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

func (r *credentialRepo) DeleteByPoolID(ctx context.Context, poolID uint) (int64, error) {
	result := r.db.WithContext(ctx).Where("pool_id = ?", poolID).Delete(&models.Credential{})
	return result.RowsAffected, result.Error
}

func (r *credentialRepo) UpdateStatusByIDs(ctx context.Context, poolID uint, ids []uint, status string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.Credential{}).Where("pool_id = ? AND id IN ?", poolID, ids).Updates(map[string]any{"status": status})
	return result.RowsAffected, result.Error
}

func (r *credentialRepo) DeleteByIDs(ctx context.Context, poolID uint, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Where("pool_id = ? AND id IN ?", poolID, ids).Delete(&models.Credential{})
	return result.RowsAffected, result.Error
}

func (r *credentialRepo) ExistingHashes(ctx context.Context, poolID uint, hashes []string) (map[string]bool, error) {
	existing := map[string]bool{}
	if len(hashes) == 0 {
		return existing, nil
	}
	var found []string
	if err := r.db.WithContext(ctx).Model(&models.Credential{}).
		Where("pool_id = ? AND key_hash IN ?", poolID, hashes).
		Pluck("key_hash", &found).Error; err != nil {
		return nil, err
	}
	for _, h := range found {
		existing[h] = true
	}
	return existing, nil
}
