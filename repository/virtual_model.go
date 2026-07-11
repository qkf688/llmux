package repository

import (
	"context"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// VirtualModelRepo 封装 VirtualModel 实体的数据访问。
type VirtualModelRepo interface {
	// List 返回所有虚拟模型。
	List(ctx context.Context) ([]models.VirtualModel, error)
	// Get 根据 ID 获取虚拟模型。
	Get(ctx context.Context, id uint) (*models.VirtualModel, error)
	// GetByName 根据名称获取虚拟模型。
	GetByName(ctx context.Context, name string) (*models.VirtualModel, error)
	// ExistsByName 判断指定名称是否存在。
	ExistsByName(ctx context.Context, name string) (bool, error)
	// ExistsByNameExceptID 判断名称是否被其他 ID 占用。
	ExistsByNameExceptID(ctx context.Context, name string, exceptID uint) (bool, error)
	// Create 创建虚拟模型。
	Create(ctx context.Context, vm *models.VirtualModel) error
	// Update 根据 ID 更新虚拟模型。
	Update(ctx context.Context, id uint, vm *models.VirtualModel) error
	// Delete 根据 ID 软删除虚拟模型。
	Delete(ctx context.Context, id uint) (int64, error)
}

// VirtualModelMappingRepo 封装 VirtualModelMapping 实体的数据访问。
// 删除类方法一律 Unscoped 硬删（唯一索引 + 重建语义要求）。
type VirtualModelMappingRepo interface {
	// ListByVirtualModel 返回指定虚拟模型的所有映射。
	ListByVirtualModel(ctx context.Context, virtualModelID uint) ([]models.VirtualModelMapping, error)
	// ListByVirtualModelAndRealModelIDs 返回指定 VM 下、真实模型 ID 落在集合内的映射。
	ListByVirtualModelAndRealModelIDs(ctx context.Context, virtualModelID uint, realModelIDs []uint) ([]models.VirtualModelMapping, error)
	// Get 根据 ID 获取映射。
	Get(ctx context.Context, id uint) (*models.VirtualModelMapping, error)
	// GetByVirtualModelAndID 根据映射 ID + 所属虚拟模型 ID 获取映射。
	GetByVirtualModelAndID(ctx context.Context, virtualModelID, mappingID uint) (*models.VirtualModelMapping, error)
	// CountByPair 统计 (virtual_model_id, real_model_id) 映射数量。
	CountByPair(ctx context.Context, virtualModelID, realModelID uint) (int64, error)
	// Create 创建映射。
	Create(ctx context.Context, mapping *models.VirtualModelMapping) error
	// Update 根据 ID 更新映射。
	Update(ctx context.Context, id uint, mapping *models.VirtualModelMapping) error
	// Delete 硬删指定 ID 的映射。
	Delete(ctx context.Context, id uint) (int64, error)
	// DeleteByVirtualModelAndID 硬删指定 VM 下的单条映射。
	DeleteByVirtualModelAndID(ctx context.Context, virtualModelID, mappingID uint) (int64, error)
	// BatchDelete 硬删指定虚拟模型下的映射（ids 为空时不删除任何行，由调用方拦截）。
	BatchDelete(ctx context.Context, virtualModelID uint, ids []uint) (int64, error)
	// HardDeleteByVirtualModelID 硬删指定虚拟模型下的全部映射。
	HardDeleteByVirtualModelID(ctx context.Context, virtualModelID uint) (int64, error)
}

// NewVirtualModelRepo 创建 VirtualModelRepo 实现。
func NewVirtualModelRepo(db *gorm.DB) VirtualModelRepo {
	return &virtualModelRepo{db: db}
}

// NewVirtualModelMappingRepo 创建 VirtualModelMappingRepo 实现。
func NewVirtualModelMappingRepo(db *gorm.DB) VirtualModelMappingRepo {
	return &virtualModelMappingRepo{db: db}
}

type virtualModelRepo struct {
	db *gorm.DB
}

func (r *virtualModelRepo) List(ctx context.Context) ([]models.VirtualModel, error) {
	var vms []models.VirtualModel
	if err := r.db.WithContext(ctx).Find(&vms).Error; err != nil {
		return nil, err
	}
	return vms, nil
}

func (r *virtualModelRepo) Get(ctx context.Context, id uint) (*models.VirtualModel, error) {
	var vm models.VirtualModel
	if err := r.db.WithContext(ctx).First(&vm, id).Error; err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *virtualModelRepo) GetByName(ctx context.Context, name string) (*models.VirtualModel, error) {
	var vm models.VirtualModel
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&vm).Error; err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *virtualModelRepo) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.VirtualModel{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *virtualModelRepo) ExistsByNameExceptID(ctx context.Context, name string, exceptID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.VirtualModel{}).
		Where("name = ? AND id != ?", name, exceptID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *virtualModelRepo) Create(ctx context.Context, vm *models.VirtualModel) error {
	return r.db.WithContext(ctx).Create(vm).Error
}

func (r *virtualModelRepo) Update(ctx context.Context, id uint, vm *models.VirtualModel) error {
	return r.db.WithContext(ctx).Model(&models.VirtualModel{}).Where("id = ?", id).Updates(vm).Error
}

func (r *virtualModelRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.VirtualModel{}, id)
	return result.RowsAffected, result.Error
}

type virtualModelMappingRepo struct {
	db *gorm.DB
}

func (r *virtualModelMappingRepo) ListByVirtualModel(ctx context.Context, virtualModelID uint) ([]models.VirtualModelMapping, error) {
	var mappings []models.VirtualModelMapping
	if err := r.db.WithContext(ctx).
		Where("virtual_model_id = ?", virtualModelID).
		Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *virtualModelMappingRepo) ListByVirtualModelAndRealModelIDs(ctx context.Context, virtualModelID uint, realModelIDs []uint) ([]models.VirtualModelMapping, error) {
	if len(realModelIDs) == 0 {
		return []models.VirtualModelMapping{}, nil
	}
	var mappings []models.VirtualModelMapping
	if err := r.db.WithContext(ctx).
		Where("virtual_model_id = ? AND real_model_id IN ?", virtualModelID, realModelIDs).
		Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *virtualModelMappingRepo) Get(ctx context.Context, id uint) (*models.VirtualModelMapping, error) {
	var mapping models.VirtualModelMapping
	if err := r.db.WithContext(ctx).First(&mapping, id).Error; err != nil {
		return nil, err
	}
	return &mapping, nil
}

func (r *virtualModelMappingRepo) GetByVirtualModelAndID(ctx context.Context, virtualModelID, mappingID uint) (*models.VirtualModelMapping, error) {
	var mapping models.VirtualModelMapping
	if err := r.db.WithContext(ctx).
		Where("id = ? AND virtual_model_id = ?", mappingID, virtualModelID).
		First(&mapping).Error; err != nil {
		return nil, err
	}
	return &mapping, nil
}

func (r *virtualModelMappingRepo) CountByPair(ctx context.Context, virtualModelID, realModelID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.VirtualModelMapping{}).
		Where("virtual_model_id = ? AND real_model_id = ?", virtualModelID, realModelID).
		Count(&count).Error
	return count, err
}

func (r *virtualModelMappingRepo) Create(ctx context.Context, mapping *models.VirtualModelMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

func (r *virtualModelMappingRepo) Update(ctx context.Context, id uint, mapping *models.VirtualModelMapping) error {
	return r.db.WithContext(ctx).Model(&models.VirtualModelMapping{}).Where("id = ?", id).Updates(mapping).Error
}

func (r *virtualModelMappingRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&models.VirtualModelMapping{})
	return result.RowsAffected, result.Error
}

func (r *virtualModelMappingRepo) DeleteByVirtualModelAndID(ctx context.Context, virtualModelID, mappingID uint) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().
		Where("id = ? AND virtual_model_id = ?", mappingID, virtualModelID).
		Delete(&models.VirtualModelMapping{})
	return result.RowsAffected, result.Error
}

func (r *virtualModelMappingRepo) BatchDelete(ctx context.Context, virtualModelID uint, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Unscoped().
		Where("virtual_model_id = ? AND id IN ?", virtualModelID, ids).
		Delete(&models.VirtualModelMapping{})
	return result.RowsAffected, result.Error
}

func (r *virtualModelMappingRepo) HardDeleteByVirtualModelID(ctx context.Context, virtualModelID uint) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().
		Where("virtual_model_id = ?", virtualModelID).
		Delete(&models.VirtualModelMapping{})
	return result.RowsAffected, result.Error
}