package virtualmodel

import (
	"context"
	"sync"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// Service 虚拟模型服务。
type Service struct {
	db *gorm.DB

	// 轮询策略的状态管理。
	roundRobinState map[uint]int // virtualModelID -> 当前索引
	mu              sync.RWMutex
}

var (
	serviceInstance *Service
	serviceOnce     sync.Once
)

// NewService 创建或获取虚拟模型服务单例。
func NewService(db *gorm.DB) *Service {
	serviceOnce.Do(func() {
		serviceInstance = &Service{
			db:              db,
			roundRobinState: make(map[uint]int),
		}
	})
	return serviceInstance
}

// Default 返回绑定默认数据库连接的虚拟模型服务单例。
// 供不应感知 *gorm.DB 装配细节的调用方（如聊天主路径）使用。
func Default() *Service {
	return NewService(models.DB)
}

// ResetSingletonForTest 重置单例（仅用于测试）。
func ResetSingletonForTest() {
	serviceInstance = nil
	serviceOnce = sync.Once{}
}

// SelectRealModel 根据虚拟模型和策略选择真实模型。
func (s *Service) SelectRealModel(ctx context.Context, virtualModel *models.VirtualModel) (*models.Model, error) {
	pool, err := s.loadCandidatePool(ctx, virtualModel.ID)
	if err != nil {
		return nil, err
	}

	sel, _ := GetSelector(virtualModel.Strategy)
	return sel.Select(ctx, s, SelectionContext{
		VirtualModelID: virtualModel.ID,
		Mappings:       pool.mappings,
		ModelByID:      pool.modelByID,
	})
}

// SelectRealModelsOrdered 根据虚拟模型和策略返回有序的真实模型列表。
// 用于支持真实模型级别的故障转移。
func (s *Service) SelectRealModelsOrdered(ctx context.Context, virtualModel *models.VirtualModel) ([]OrderedRealModel, error) {
	pool, err := s.loadCandidatePool(ctx, virtualModel.ID)
	if err != nil {
		return nil, err
	}

	sel, _ := GetSelector(virtualModel.Strategy)
	return sel.SelectOrdered(ctx, s, SelectionContext{
		VirtualModelID: virtualModel.ID,
		Mappings:       pool.mappings,
		ModelByID:      pool.modelByID,
	})
}
