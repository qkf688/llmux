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
	// Create 创建虚拟模型。
	Create(ctx context.Context, vm *models.VirtualModel) error
	// Update 根据 ID 更新虚拟模型。
	Update(ctx context.Context, id uint, vm *models.VirtualModel) error
	// Delete 根据 ID 删除虚拟模型。
	Delete(ctx context.Context, id uint) (int64, error)
}

// VirtualModelMappingRepo 封装 VirtualModelMapping 实体的数据访问。
type VirtualModelMappingRepo interface {
	// ListByVirtualModel 返回指定虚拟模型的所有映射。
	ListByVirtualModel(ctx context.Context, virtualModelID uint) ([]models.VirtualModelMapping, error)
	// Get 根据 ID 获取映射。
	Get(ctx context.Context, id uint) (*models.VirtualModelMapping, error)
	// Create 创建映射。
	Create(ctx context.Context, mapping *models.VirtualModelMapping) error
	// Update 根据 ID 更新映射。
	Update(ctx context.Context, id uint, mapping *models.VirtualModelMapping) error
	// Delete 根据 ID 删除映射。
	Delete(ctx context.Context, id uint) (int64, error)
	// BatchDelete 批量删除指定虚拟模型下的映射。
	BatchDelete(ctx context.Context, virtualModelID uint, ids []uint) (int64, error)
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

func (r *virtualModelMappingRepo) Get(ctx context.Context, id uint) (*models.VirtualModelMapping, error) {
	var mapping models.VirtualModelMapping
	if err := r.db.WithContext(ctx).First(&mapping, id).Error; err != nil {
		return nil, err
	}
	return &mapping, nil
}

func (r *virtualModelMappingRepo) Create(ctx context.Context, mapping *models.VirtualModelMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

func (r *virtualModelMappingRepo) Update(ctx context.Context, id uint, mapping *models.VirtualModelMapping) error {
	return r.db.WithContext(ctx).Model(&models.VirtualModelMapping{}).Where("id = ?", id).Updates(mapping).Error
}

func (r *virtualModelMappingRepo) Delete(ctx context.Context, id uint) (int64, error) {
	result := r.db.WithContext(ctx).Delete(&models.VirtualModelMapping{}, id)
	return result.RowsAffected, result.Error
}

func (r *virtualModelMappingRepo) BatchDelete(ctx context.Context, virtualModelID uint, ids []uint) (int64, error) {
	query := r.db.WithContext(ctx).Where("virtual_model_id = ?", virtualModelID)
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}
	result := query.Delete(&models.VirtualModelMapping{})
	return result.RowsAffected, result.Error
}
