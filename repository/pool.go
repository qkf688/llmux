package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// ErrPoolNameTaken 表示建池时撞了 pools.name 唯一索引（同名号池已存在）。
// 与 ErrApiKeyExists 同口径：查重语义走「DB 撞库翻译」而非应用层预查
// （预检查询与写入之间存在竞态窗口，uniqueIndex 才是单一真相），
// 调用方据此换名重试或映射 HTTP 冲突，而不是把它当成内部故障报 500。
var ErrPoolNameTaken = errors.New("repository: pool name already taken")

// PoolRepo 封装号池（pools）实体的数据访问。
type PoolRepo interface {
	// List 返回符合条件的号池；filter 为零值时返回全部。
	List(ctx context.Context, filter PoolFilter) ([]models.Pool, error)
	// Get 根据 ID 获取号池。
	Get(ctx context.Context, id uint) (*models.Pool, error)
	// ExistsByName 判断同名号池是否存在（Name 有唯一索引）。
	ExistsByName(ctx context.Context, name string) (bool, error)
	// Create 创建号池。
	Create(ctx context.Context, pool *models.Pool) error
	// Update 根据 ID 更新号池。struct 更新会跳过零值字段——清空 Note 必须用 UpdateFields。
	Update(ctx context.Context, id uint, pool *models.Pool) error
	// UpdateFields 按字段 map 更新（map 可显式写空串/NULL，绕过 struct 零值跳过语义）。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// Delete 根据 ID 删除号池，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// StatsByIDs 返回指定号池的凭据统计（key 数 + 状态分布）；ids 为空时返回空 map。
	// 号池列表页「健康概览」用：一次聚合查询取回多个号池的统计，避免 N+1。
	StatsByIDs(ctx context.Context, ids []uint) (map[uint]PoolStats, error)
}

// PoolStats 单个号池的凭据统计（设计定案第 2 节「号池页概览用」）。
type PoolStats struct {
	// KeyCount 该号池凭据总数（含 disabled/error，含冷却中——冷却用 CooldownUntil 表达不占状态）。
	KeyCount int64
	// StatusCount 各状态凭据数，键为 models.CredentialStatus 常量；只含非零状态。
	StatusCount map[string]int64
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

func (r *poolRepo) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Pool{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *poolRepo) Create(ctx context.Context, pool *models.Pool) error {
	err := r.db.WithContext(ctx).Create(pool).Error
	if err == nil {
		return nil
	}
	// 同名不静默：撞唯一索引翻译成 ErrPoolNameTaken，由调用方决定换名重试还是报冲突。
	if isUniqueViolationOn(err, "pools.name") {
		return fmt.Errorf("%w: %w", err, ErrPoolNameTaken)
	}
	return err
}

func (r *poolRepo) Update(ctx context.Context, id uint, pool *models.Pool) error {
	return r.db.WithContext(ctx).Model(&models.Pool{}).Where("id = ?", id).Updates(pool).Error
}

func (r *poolRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.Pool{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

// Delete 硬删号池。硬删而非软删（与 virtual_models / api_keys 口径统一）：
// pools.name 有唯一索引且不含 deleted_at，软删行不可见却仍占着名字——
// 「先删后用同名重建」会先过 ExistsByName 预检、再撞唯一索引，回 500
// 「Failed to create pool」，报错方向全错。号池删除另有引用守卫兜底
// （KeyGroupRepo.CountByPoolIDs > 0 直接 400），误删面已收窄。
func (r *poolRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Delete(&models.Pool{}, id)
	return result.RowsAffected, result.Error
}

// StatsByIDs 用一条 GROUP BY 聚合返回指定号池的凭据统计（key 数与状态分布），
// 避免号池列表逐池查凭据的 N+1。
// ids 为空或某号池无凭据时不产生对应条目。
func (r *poolRepo) StatsByIDs(ctx context.Context, ids []uint) (map[uint]PoolStats, error) {
	if len(ids) == 0 {
		return map[uint]PoolStats{}, nil
	}

	type statRow struct {
		PoolID uint
		Status string
		Count  int64
	}
	var rows []statRow
	// pool_id 直接取非空（本条查询的 ids 均为真实号池），Group 按 pool_id+status 聚合
	if err := r.db.WithContext(ctx).
		Model(&models.Credential{}).
		Select("pool_id, status, COUNT(*) AS count").
		Where("pool_id IN ?", ids).
		Group("pool_id, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make(map[uint]PoolStats, len(rows))
	for _, row := range rows {
		s := stats[row.PoolID]
		s.KeyCount += row.Count
		if s.StatusCount == nil {
			s.StatusCount = make(map[string]int64)
		}
		s.StatusCount[row.Status] = row.Count
		stats[row.PoolID] = s
	}
	return stats, nil
}
