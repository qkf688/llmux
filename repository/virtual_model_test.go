package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
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

func TestVirtualModelRepo_DeleteAllowsRecreateSameName(t *testing.T) {
	ctx := context.Background()
	db := newVMTestDB(t)
	repo := NewVirtualModelRepo(db)

	vm := &models.VirtualModel{Name: "vm-x", Strategy: "priority"}
	if err := repo.Create(ctx, vm); err != nil {
		t.Fatalf("Create: %v", err)
	}

	affected, err := repo.Delete(ctx, vm.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}

	var leftover int64
	if err := db.Unscoped().Model(&models.VirtualModel{}).
		Where("name = ?", "vm-x").Count(&leftover).Error; err != nil {
		t.Fatalf("count leftover: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("hard count after delete = %d, want 0", leftover)
	}

	// Name 唯一索引不含 deleted_at：软删残留会让同名重建撞 UNIQUE
	if err := repo.Create(ctx, &models.VirtualModel{Name: "vm-x", Strategy: "priority"}); err != nil {
		t.Fatalf("recreate with same name: %v", err)
	}
}

func TestVirtualModelMappingRepo_DeleteByRealModelIDAllowsRecreate(t *testing.T) {
	ctx := context.Background()
	db := newVMTestDB(t)
	vmRepo := NewVirtualModelRepo(db)
	mapRepo := NewVirtualModelMappingRepo(db)

	enabled := true
	var vmIDs []uint
	for _, name := range []string{"vm1", "vm2"} {
		vm := &models.VirtualModel{Name: name, Strategy: "priority"}
		if err := vmRepo.Create(ctx, vm); err != nil {
			t.Fatalf("Create VM %s: %v", name, err)
		}
		vmIDs = append(vmIDs, vm.ID)
		if err := mapRepo.Create(ctx, &models.VirtualModelMapping{
			VirtualModelID: vm.ID, RealModelID: 10, Priority: 1, Weight: 5, Enabled: &enabled,
		}); err != nil {
			t.Fatalf("Create mapping for %s: %v", name, err)
		}
	}
	// 指向其他真实模型的映射不应被误删
	if err := mapRepo.Create(ctx, &models.VirtualModelMapping{
		VirtualModelID: vmIDs[0], RealModelID: 20, Priority: 1, Weight: 5, Enabled: &enabled,
	}); err != nil {
		t.Fatalf("Create mapping for other real model: %v", err)
	}

	affected, err := mapRepo.DeleteByRealModelID(ctx, 10)
	if err != nil {
		t.Fatalf("DeleteByRealModelID: %v", err)
	}
	if affected != 2 {
		t.Errorf("affected = %d, want 2", affected)
	}

	var leftover int64
	if err := db.Unscoped().Model(&models.VirtualModelMapping{}).
		Where("real_model_id = ?", 10).Count(&leftover).Error; err != nil {
		t.Fatalf("count leftover: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("hard count after delete = %d, want 0", leftover)
	}

	var others int64
	if err := db.Model(&models.VirtualModelMapping{}).
		Where("real_model_id = ?", 20).Count(&others).Error; err != nil {
		t.Fatalf("count others: %v", err)
	}
	if others != 1 {
		t.Fatalf("other real model mappings = %d, want 1", others)
	}

	// 硬删后可重建（unique index 不含 deleted_at）
	if err := mapRepo.Create(ctx, &models.VirtualModelMapping{
		VirtualModelID: vmIDs[0], RealModelID: 10, Priority: 1, Weight: 5, Enabled: &enabled,
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

// UpdateFields 必须能写入零值（GORM 结构体 Updates 会跳过零值）。
func TestVirtualModelRepo_UpdateFields_WritesZeroValues(t *testing.T) {
	ctx := context.Background()
	db := newVMTestDB(t)
	repo := NewVirtualModelRepo(db)

	enabled := true
	vm := &models.VirtualModel{Name: "vm-zero", Description: "desc", MaxRetry: 5, TimeOut: 30, Enabled: &enabled}
	if err := repo.Create(ctx, vm); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 清空 Description，MaxRetry/TimeOut 设 0
	updates := map[string]any{
		"description": "",
		"max_retry":   0,
		"time_out":    0,
	}
	if _, err := repo.UpdateFields(ctx, vm.ID, updates); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}

	got, err := repo.Get(ctx, vm.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "" {
		t.Fatalf("Description = %q, want empty", got.Description)
	}
	if got.MaxRetry != 0 {
		t.Fatalf("MaxRetry = %d, want 0", got.MaxRetry)
	}
	if got.TimeOut != 0 {
		t.Fatalf("TimeOut = %d, want 0", got.TimeOut)
	}
}

func TestVirtualModelMappingRepo_UpdateFields_WritesZeroPriority(t *testing.T) {
	ctx := context.Background()
	db := newVMTestDB(t)
	vmRepo := NewVirtualModelRepo(db)
	mapRepo := NewVirtualModelMappingRepo(db)

	vm := &models.VirtualModel{Name: "vm-mp"}
	if err := vmRepo.Create(ctx, vm); err != nil {
		t.Fatalf("Create VM: %v", err)
	}
	enabled := true
	mapping := &models.VirtualModelMapping{
		VirtualModelID: vm.ID, RealModelID: 10, Priority: 5, Weight: 3, Enabled: &enabled,
	}
	if err := mapRepo.Create(ctx, mapping); err != nil {
		t.Fatalf("Create mapping: %v", err)
	}

	// Priority=0, Weight=0 是合法零值，必须能写入
	updates := map[string]any{"priority": 0, "weight": 0}
	if _, err := mapRepo.UpdateFields(ctx, mapping.ID, updates); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}

	got, err := mapRepo.Get(ctx, mapping.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Priority != 0 {
		t.Fatalf("Priority = %d, want 0", got.Priority)
	}
	if got.Weight != 0 {
		t.Fatalf("Weight = %d, want 0", got.Weight)
	}
}
