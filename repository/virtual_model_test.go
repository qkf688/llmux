package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newVMTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.VirtualModel{}, &models.VirtualModelMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestVirtualModelMappingRepo_HardDeleteAllowsRecreate(t *testing.T) {
	ctx := context.Background()
	db := newVMTestDB(t)
	vmRepo := NewVirtualModelRepo(db)
	mapRepo := NewVirtualModelMappingRepo(db)

	vm := &models.VirtualModel{Name: "vm1", Strategy: "priority"}
	if err := vmRepo.Create(ctx, vm); err != nil {
		t.Fatalf("Create VM: %v", err)
	}

	enabled := true
	mapping := &models.VirtualModelMapping{
		VirtualModelID: vm.ID,
		RealModelID:    10,
		Priority:       1,
		Weight:         5,
		Enabled:        &enabled,
	}
	if err := mapRepo.Create(ctx, mapping); err != nil {
		t.Fatalf("Create mapping: %v", err)
	}

	affected, err := mapRepo.DeleteByVirtualModelAndID(ctx, vm.ID, mapping.ID)
	if err != nil {
		t.Fatalf("DeleteByVirtualModelAndID: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}

	var hardCount int64
	if err := db.Unscoped().Model(&models.VirtualModelMapping{}).
		Where("virtual_model_id = ?", vm.ID).Count(&hardCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if hardCount != 0 {
		t.Fatalf("hard count after delete = %d, want 0", hardCount)
	}

	// 硬删后可重建（unique index）
	if err := mapRepo.Create(ctx, &models.VirtualModelMapping{
		VirtualModelID: vm.ID,
		RealModelID:    10,
		Priority:       1,
		Weight:         5,
		Enabled:        &enabled,
	}); err != nil {
		t.Fatalf("recreate after hard delete: %v", err)
	}
}

func TestVirtualModelMappingRepo_HardDeleteByVirtualModelID(t *testing.T) {
	ctx := context.Background()
	db := newVMTestDB(t)
	vmRepo := NewVirtualModelRepo(db)
	mapRepo := NewVirtualModelMappingRepo(db)

	vm := &models.VirtualModel{Name: "vm-cascade"}
	_ = vmRepo.Create(ctx, vm)
	enabled := true
	_ = mapRepo.Create(ctx, &models.VirtualModelMapping{VirtualModelID: vm.ID, RealModelID: 1, Enabled: &enabled})
	_ = mapRepo.Create(ctx, &models.VirtualModelMapping{VirtualModelID: vm.ID, RealModelID: 2, Enabled: &enabled})

	n, err := mapRepo.HardDeleteByVirtualModelID(ctx, vm.ID)
	if err != nil {
		t.Fatalf("HardDeleteByVirtualModelID: %v", err)
	}
	if n != 2 {
		t.Fatalf("deleted = %d, want 2", n)
	}

	list, err := mapRepo.ListByVirtualModel(ctx, vm.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("list len = %d after cascade hard delete", len(list))
	}
}

func TestVirtualModelMappingRepo_BatchDelete_EmptyIDs(t *testing.T) {
	ctx := context.Background()
	mapRepo := NewVirtualModelMappingRepo(newVMTestDB(t))
	n, err := mapRepo.BatchDelete(ctx, 1, nil)
	if err != nil {
		t.Fatalf("BatchDelete empty: %v", err)
	}
	if n != 0 {
		t.Fatalf("empty ids should delete 0, got %d", n)
	}
}

func TestVirtualModelRepo_ExistsByName(t *testing.T) {
	ctx := context.Background()
	repo := NewVirtualModelRepo(newVMTestDB(t))
	_ = repo.Create(ctx, &models.VirtualModel{Name: "a"})

	ok, err := repo.ExistsByName(ctx, "a")
	if err != nil || !ok {
		t.Fatalf("ExistsByName a: ok=%v err=%v", ok, err)
	}
	ok, err = repo.ExistsByNameExceptID(ctx, "a", 999)
	if err != nil || !ok {
		t.Fatalf("ExistsByNameExceptID other: ok=%v err=%v", ok, err)
	}
	vm, _ := repo.GetByName(ctx, "a")
	ok, err = repo.ExistsByNameExceptID(ctx, "a", vm.ID)
	if err != nil || ok {
		t.Fatalf("ExistsByNameExceptID self: ok=%v err=%v", ok, err)
	}
	_, err = repo.Get(ctx, 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Get missing: %v", err)
	}
}