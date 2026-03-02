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

	switch virtualModel.Strategy {
	case "priority":
		return s.selectByPriority(pool.mappings, pool.modelByID)
	case "round_robin":
		return s.selectByRoundRobin(virtualModel.ID, pool.mappings, pool.modelByID)
	case "random":
		return s.selectByRandom(pool.mappings, pool.modelByID)
	default:
		return s.selectByPriority(pool.mappings, pool.modelByID)
	}
}

// SelectRealModelsOrdered 根据虚拟模型和策略返回有序的真实模型列表。
// 用于支持真实模型级别的故障转移。
func (s *Service) SelectRealModelsOrdered(ctx context.Context, virtualModel *models.VirtualModel) ([]OrderedRealModel, error) {
	pool, err := s.loadCandidatePool(ctx, virtualModel.ID)
	if err != nil {
		return nil, err
	}

	switch virtualModel.Strategy {
	case "priority":
		return s.selectOrderedByPriority(pool.mappings, pool.modelByID)
	case "round_robin":
		return s.selectOrderedByRoundRobin(virtualModel.ID, pool.mappings, pool.modelByID)
	case "random":
		return s.selectOrderedByRandom(pool.mappings, pool.modelByID)
	default:
		return s.selectOrderedByPriority(pool.mappings, pool.modelByID)
	}
}
