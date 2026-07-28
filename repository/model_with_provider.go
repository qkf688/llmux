package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ModelCapabilityFilter 关联能力筛选：字段为 true 时要求关联具备该能力。
// 全部为 false 时不附加任何能力条件（对应「关闭严格能力匹配」）。
type ModelCapabilityFilter struct {
	ToolCall         bool
	StructuredOutput bool
	Image            bool
}

// ModelWithProviderRepo 封装模型-供应商关联的数据访问（接口形状对齐现网 handler 查询）。
type ModelWithProviderRepo interface {
	// ListByModelID 返回指定模型的全部关联。
	ListByModelID(ctx context.Context, modelID uint) ([]models.ModelWithProvider, error)
	// ListEnabledByModelID 返回指定模型下 status=true 的关联，并按 caps 附加能力过滤。
	ListEnabledByModelID(ctx context.Context, modelID uint, caps ModelCapabilityFilter) ([]models.ModelWithProvider, error)
	// ListByStatus 按启用状态返回关联；status 为 nil 时返回全部。
	ListByStatus(ctx context.Context, status *bool) ([]models.ModelWithProvider, error)
	// ListAll 返回全部关联。
	ListAll(ctx context.Context) ([]models.ModelWithProvider, error)
	// Get 根据 ID 获取关联。
	Get(ctx context.Context, id uint) (*models.ModelWithProvider, error)
	// Create 创建关联。
	Create(ctx context.Context, assoc *models.ModelWithProvider) error
	// Update 根据 ID 更新关联字段（GORM Updates：结构体零值字段不写入）。
	Update(ctx context.Context, id uint, updates models.ModelWithProvider) error
	// UpdateFields 按列名更新指定关联（可写入零值），返回受影响行数。
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)
	// ResetConsecutiveFailures 将非零的连续失败计数清零，返回受影响行数。
	ResetConsecutiveFailures(ctx context.Context, id uint) (int64, error)
	// IncrementConsecutiveFailures 原子自增连续失败计数（UPDATE consecutive_failures = consecutive_failures + 1），返回受影响行数。
	IncrementConsecutiveFailures(ctx context.Context, id uint) (int64, error)
	// IncreaseWeight 原子自增 weight（带上限钳制：已超上限则不变，否则 MIN(weight+step, max)），返回受影响行数。
	IncreaseWeight(ctx context.Context, id uint, step, max int) (int64, error)
	// IncreasePriority 原子自增 priority（带上限钳制：已超上限则不变，否则 MIN(priority+step, max)），返回受影响行数。
	IncreasePriority(ctx context.Context, id uint, step, max int) (int64, error)
	// DecayWeight 原子自减 weight（带下限钳制：已在下限则不变，否则 MAX(weight-step, floor)），返回受影响行数。
	DecayWeight(ctx context.Context, id uint, step, floor int) (int64, error)
	// DecayPriority 原子自减 priority（带下限钳制：已在下限则不变，否则 MAX(priority-step, floor)），返回受影响行数。
	DecayPriority(ctx context.Context, id uint, step, floor int) (int64, error)
	// Delete 根据 ID 删除关联，返回受影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// DeleteByIDs 批量删除关联，返回受影响行数。
	DeleteByIDs(ctx context.Context, ids []uint) (int64, error)
	// UpdateByIDs 批量更新关联字段（GORM Updates：结构体零值字段不写入），返回受影响行数。
	UpdateByIDs(ctx context.Context, ids []uint, updates models.ModelWithProvider) (int64, error)
	// UpdateFieldsByIDs 按列名批量更新关联（可写入零值），返回受影响行数。
	UpdateFieldsByIDs(ctx context.Context, ids []uint, fields map[string]any) (int64, error)
	// DeleteByProviderID 删除指定供应商下的全部关联，返回受影响行数。
	DeleteByProviderID(ctx context.Context, providerID uint) (int64, error)
	// DeleteByModelID 删除指定模型下的全部关联，返回受影响行数。
	DeleteByModelID(ctx context.Context, modelID uint) (int64, error)
}

// NewModelWithProviderRepo 创建 ModelWithProviderRepo 实现。
func NewModelWithProviderRepo(db *gorm.DB) ModelWithProviderRepo {
	return &modelWithProviderRepo{db: db}
}

type modelWithProviderRepo struct {
	db *gorm.DB
}

func (r *modelWithProviderRepo) ListByModelID(ctx context.Context, modelID uint) ([]models.ModelWithProvider, error) {
	var rows []models.ModelWithProvider
	if err := r.db.WithContext(ctx).Where("model_id = ?", modelID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *modelWithProviderRepo) ListEnabledByModelID(ctx context.Context, modelID uint, caps ModelCapabilityFilter) ([]models.ModelWithProvider, error) {
	query := r.db.WithContext(ctx).Where("model_id = ? AND status = ?", modelID, true)
	if caps.ToolCall {
		query = query.Where("tool_call = ?", true)
	}
	if caps.StructuredOutput {
		query = query.Where("structured_output = ?", true)
	}
	if caps.Image {
		query = query.Where("image = ?", true)
	}

	var rows []models.ModelWithProvider
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *modelWithProviderRepo) ListByStatus(ctx context.Context, status *bool) ([]models.ModelWithProvider, error) {
	query := r.db.WithContext(ctx).Model(&models.ModelWithProvider{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var rows []models.ModelWithProvider
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *modelWithProviderRepo) ListAll(ctx context.Context) ([]models.ModelWithProvider, error) {
	var rows []models.ModelWithProvider
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *modelWithProviderRepo) Get(ctx context.Context, id uint) (*models.ModelWithProvider, error) {
	var row models.ModelWithProvider
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *modelWithProviderRepo) Create(ctx context.Context, assoc *models.ModelWithProvider) error {
	return r.db.WithContext(ctx).Create(assoc).Error
}

func (r *modelWithProviderRepo) Update(ctx context.Context, id uint, updates models.ModelWithProvider) error {
	return r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).Where("id = ?", id).Updates(updates).Error
}

func (r *modelWithProviderRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).Where("id = ?", id).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) ResetConsecutiveFailures(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).
		Where("id = ? AND consecutive_failures != 0", id).
		Update("consecutive_failures", 0)
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) IncrementConsecutiveFailures(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).
		Where("id = ?", id).
		Update("consecutive_failures", gorm.Expr("consecutive_failures + 1"))
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) IncreaseWeight(ctx context.Context, id uint, step, max int) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).
		Where("id = ? AND weight < ?", id, max).
		Update("weight", gorm.Expr("MIN(weight + ?, ?)", step, max))
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) IncreasePriority(ctx context.Context, id uint, step, max int) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).
		Where("id = ? AND priority < ?", id, max).
		Update("priority", gorm.Expr("MIN(priority + ?, ?)", step, max))
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DecayWeight(ctx context.Context, id uint, step, floor int) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).
		Where("id = ? AND weight > ?", id, floor).
		Update("weight", gorm.Expr("MAX(weight - ?, ?)", step, floor))
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DecayPriority(ctx context.Context, id uint, step, floor int) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).
		Where("id = ? AND priority > ?", id, floor).
		Update("priority", gorm.Expr("MAX(priority - ?, ?)", step, floor))
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DeleteByIDs(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) UpdateByIDs(ctx context.Context, ids []uint, updates models.ModelWithProvider) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).Where("id IN ?", ids).Updates(updates)
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) UpdateFieldsByIDs(ctx context.Context, ids []uint, fields map[string]any) (int64, error) {
	if len(ids) == 0 || len(fields) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&models.ModelWithProvider{}).Where("id IN ?", ids).Updates(fields)
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DeleteByProviderID(ctx context.Context, providerID uint) (int64, error) {
	result := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}

func (r *modelWithProviderRepo) DeleteByModelID(ctx context.Context, modelID uint) (int64, error) {
	result := r.db.WithContext(ctx).Where("model_id = ?", modelID).Delete(&models.ModelWithProvider{})
	return result.RowsAffected, result.Error
}
