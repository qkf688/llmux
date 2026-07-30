package service

import (
	"context"
	"fmt"
	"github.com/atopos31/llmio/service/internal/testutil"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/virtualmodel"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func resetVirtualModelServiceSingleton() {
	virtualmodel.ResetSingletonForTest()
}

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	resetVirtualModelServiceSingleton()

	dsn := fmt.Sprintf("file:virtual_model_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	sqlDB := testutil.ConfigureSQLiteForSingleConn(t, db)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	// 自动迁移表结构
	err = db.AutoMigrate(
		&models.VirtualModel{},
		&models.VirtualModelMapping{},
		&models.Model{},
		&models.Provider{},
		&models.ModelWithProvider{},
	)
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

// createTestModels 创建测试用的模型数据
func createTestModels(t *testing.T, db *gorm.DB) ([]models.Model, []models.VirtualModel, []models.VirtualModelMapping) {
	// 创建真实模型
	realModels := []models.Model{
		{Name: "gpt-4", Remark: "OpenAI GPT-4"},
		{Name: "claude-3", Remark: "Anthropic Claude 3"},
		{Name: "gemini-pro", Remark: "Google Gemini Pro"},
	}

	for i := range realModels {
		if err := db.Create(&realModels[i]).Error; err != nil {
			t.Fatalf("failed to create real model: %v", err)
		}
	}

	// 创建虚拟模型
	virtualModels := []models.VirtualModel{
		{
			Name:        "smart-chat",
			Description: "智能聊天模型集合",
			Strategy:    "round_robin",
			MaxRetry:    3,
			TimeOut:     30,
		},
		{
			Name:        "backup-chat",
			Description: "备份聊天模型集合",
			Strategy:    "priority",
			MaxRetry:    2,
			TimeOut:     60,
		},
	}

	for i := range virtualModels {
		if err := db.Create(&virtualModels[i]).Error; err != nil {
			t.Fatalf("failed to create virtual model: %v", err)
		}
	}

	// 创建映射关系
	mappings := []models.VirtualModelMapping{
		{
			VirtualModelID: virtualModels[0].ID,
			RealModelID:    realModels[0].ID,
			Priority:       10,
			Weight:         5,
			Enabled:        boolPtr(true),
		},
		{
			VirtualModelID: virtualModels[0].ID,
			RealModelID:    realModels[1].ID,
			Priority:       10,
			Weight:         5,
			Enabled:        boolPtr(true),
		},
		{
			VirtualModelID: virtualModels[0].ID,
			RealModelID:    realModels[2].ID,
			Priority:       10,
			Weight:         5,
			Enabled:        boolPtr(true),
		},
		{
			VirtualModelID: virtualModels[1].ID,
			RealModelID:    realModels[1].ID,
			Priority:       15,
			Weight:         3,
			Enabled:        boolPtr(true),
		},
		{
			VirtualModelID: virtualModels[1].ID,
			RealModelID:    realModels[2].ID,
			Priority:       10,
			Weight:         7,
			Enabled:        boolPtr(true),
		},
	}

	for i := range mappings {
		if err := db.Create(&mappings[i]).Error; err != nil {
			t.Fatalf("failed to create mapping: %v", err)
		}
	}

	return realModels, virtualModels, mappings
}

func boolPtr(b bool) *bool {
	return &b
}

// TestVirtualModelService_RoundRobin 测试轮询策略
func TestVirtualModelService_RoundRobin(t *testing.T) {
	db := setupTestDB(t)
	realModels, virtualModels, _ := createTestModels(t, db)

	service := NewVirtualModelService(db)
	ctx := context.Background()

	virtualModel := &virtualModels[0] // smart-chat 使用 round_robin 策略

	// 测试多次调用应该按顺序返回不同模型
	expectedOrder := []uint{
		realModels[0].ID, // 第一次调用
		realModels[1].ID, // 第二次调用
		realModels[2].ID, // 第三次调用
		realModels[0].ID, // 第四次调用（回到开始）
		realModels[1].ID, // 第五次调用
	}

	selectedModels := make([]uint, 0, len(expectedOrder))

	for i := 0; i < len(expectedOrder); i++ {
		selectedModel, err := service.SelectRealModel(ctx, virtualModel)
		if err != nil {
			t.Fatalf("SelectRealModel failed on iteration %d: %v", i, err)
		}

		selectedModels = append(selectedModels, selectedModel.ID)

		// 验证选择的模型是否符合预期
		if selectedModel.ID != expectedOrder[i] {
			t.Errorf("Round robin selection failed at iteration %d: expected model ID %d, got %d",
				i, expectedOrder[i], selectedModel.ID)
		}
	}

	// 验证完整的选择序列
	t.Logf("Round robin selection sequence: %v", selectedModels)
}

// TestVirtualModelService_Priority 测试优先级策略
func TestVirtualModelService_Priority(t *testing.T) {
	db := setupTestDB(t)
	realModels, virtualModels, _ := createTestModels(t, db)

	service := NewVirtualModelService(db)
	ctx := context.Background()

	virtualModel := &virtualModels[1] // backup-chat 使用 priority 策略

	// 测试多次调用应该总是返回最高优先级的模型
	// claude-3 (priority 15) 应该总是被选中
	for i := 0; i < 5; i++ {
		selectedModel, err := service.SelectRealModel(ctx, virtualModel)
		if err != nil {
			t.Fatalf("SelectRealModel failed on iteration %d: %v", i, err)
		}

		if selectedModel.ID != realModels[1].ID {
			t.Errorf("Priority selection failed at iteration %d: expected model ID %d (claude-3), got %d (%s)",
				i, realModels[1].ID, selectedModel.ID, selectedModel.Name)
		}
	}
}

// TestVirtualModelService_Random 测试随机策略
func TestVirtualModelService_Random(t *testing.T) {
	db := setupTestDB(t)
	realModels, _, _ := createTestDBForRandom(t, db)

	service := NewVirtualModelService(db)
	ctx := context.Background()

	// 创建使用随机策略的虚拟模型
	randomVM := models.VirtualModel{
		Name:     "random-chat",
		Strategy: "random",
	}
	if err := db.Create(&randomVM).Error; err != nil {
		t.Fatalf("failed to create random virtual model: %v", err)
	}

	// 创建映射关系（都设为相同优先级和权重）
	randomMappings := []models.VirtualModelMapping{
		{
			VirtualModelID: randomVM.ID,
			RealModelID:    realModels[0].ID,
			Priority:       10,
			Weight:         1,
			Enabled:        boolPtr(true),
		},
		{
			VirtualModelID: randomVM.ID,
			RealModelID:    realModels[1].ID,
			Priority:       10,
			Weight:         1,
			Enabled:        boolPtr(true),
		},
	}

	for i := range randomMappings {
		if err := db.Create(&randomMappings[i]).Error; err != nil {
			t.Fatalf("failed to create random mapping: %v", err)
		}
	}

	// 测试多次调用应该返回不同的模型（随机性）
	selectedCounts := make(map[uint]int)

	for i := 0; i < 20; i++ {
		selectedModel, err := service.SelectRealModel(ctx, &randomVM)
		if err != nil {
			t.Fatalf("SelectRealModel failed on iteration %d: %v", i, err)
		}
		selectedCounts[selectedModel.ID]++
	}

	// 验证所有模型都被选中过（随机性测试）
	if len(selectedCounts) < 2 {
		t.Errorf("Random selection should have selected at least 2 different models, got %d", len(selectedCounts))
	}

	t.Logf("Random selection distribution: %v", selectedCounts)
}

// createTestDBForRandom 为随机测试创建专用数据
func createTestDBForRandom(t *testing.T, db *gorm.DB) ([]models.Model, []models.VirtualModel, []models.VirtualModelMapping) {
	// 创建真实模型
	realModels := []models.Model{
		{Name: "model-a", Remark: "Model A"},
		{Name: "model-b", Remark: "Model B"},
	}

	for i := range realModels {
		if err := db.Create(&realModels[i]).Error; err != nil {
			t.Fatalf("failed to create real model: %v", err)
		}
	}

	return realModels, make([]models.VirtualModel, 0), make([]models.VirtualModelMapping, 0)
}

// TestVirtualModelService_DisabledMappings 测试禁用映射的处理
func TestVirtualModelService_DisabledMappings(t *testing.T) {
	db := setupTestDB(t)
	realModels, _, _ := createTestModels(t, db)

	service := NewVirtualModelService(db)
	ctx := context.Background()

	// 创建虚拟模型
	testVM := models.VirtualModel{
		Name:     "test-disabled",
		Strategy: "round_robin",
	}
	if err := db.Create(&testVM).Error; err != nil {
		t.Fatalf("failed to create test virtual model: %v", err)
	}

	// 创建映射关系，其中一些被禁用
	mappings := []models.VirtualModelMapping{
		{
			VirtualModelID: testVM.ID,
			RealModelID:    realModels[0].ID,
			Priority:       10,
			Weight:         5,
			Enabled:        boolPtr(true),
		},
		{
			VirtualModelID: testVM.ID,
			RealModelID:    realModels[1].ID,
			Priority:       10,
			Weight:         5,
			Enabled:        boolPtr(false), // 禁用
		},
		{
			VirtualModelID: testVM.ID,
			RealModelID:    realModels[2].ID,
			Priority:       10,
			Weight:         5,
			Enabled:        boolPtr(true),
		},
	}

	for i := range mappings {
		if err := db.Create(&mappings[i]).Error; err != nil {
			t.Fatalf("failed to create mapping: %v", err)
		}
	}

	// 测试应该只从启用的映射中选择
	validModelIDs := map[uint]bool{
		realModels[0].ID: true,
		realModels[2].ID: true,
	}

	for i := 0; i < 10; i++ {
		selectedModel, err := service.SelectRealModel(ctx, &testVM)
		if err != nil {
			t.Fatalf("SelectRealModel failed on iteration %d: %v", i, err)
		}

		if !validModelIDs[selectedModel.ID] {
			t.Errorf("Selected disabled model ID %d on iteration %d", selectedModel.ID, i)
		}
	}
}

// TestVirtualModelService_EmptyMappings 测试空映射处理
func TestVirtualModelService_EmptyMappings(t *testing.T) {
	db := setupTestDB(t)

	service := NewVirtualModelService(db)
	ctx := context.Background()

	// 创建没有映射关系的虚拟模型
	emptyVM := models.VirtualModel{
		Name:     "empty-test",
		Strategy: "round_robin",
	}
	if err := db.Create(&emptyVM).Error; err != nil {
		t.Fatalf("failed to create empty virtual model: %v", err)
	}

	// 应该返回错误
	_, err := service.SelectRealModel(ctx, &emptyVM)
	if err == nil {
		t.Error("Expected error when selecting from virtual model with no mappings, got nil")
	}
}

// TestVirtualModelService_NonExistentModel 测试不存在的虚拟模型
func TestVirtualModelService_NonExistentModel(t *testing.T) {
	db := setupTestDB(t)

	service := NewVirtualModelService(db)
	ctx := context.Background()

	// 创建一个不存在的虚拟模型引用
	nonExistentVM := &models.VirtualModel{
		Model: gorm.Model{ID: 99999},
		Name:  "non-existent",
	}

	// 应该返回错误
	_, err := service.SelectRealModel(ctx, nonExistentVM)
	if err == nil {
		t.Error("Expected error when selecting from non-existent virtual model, got nil")
	}
}
