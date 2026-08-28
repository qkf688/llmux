package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pool{}, &models.Credential{}, &models.Endpoint{}, &models.KeyGroup{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestPoolRepo_CRUD 锁定号池增删改查。
func TestPoolRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewPoolRepo(newChannelTestDB(t))

	pool := &models.Pool{Name: "主池", Note: "生产密钥池"}
	if err := repo.Create(ctx, pool); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pool.ID == 0 {
		t.Fatal("ID 未回填")
	}

	got, err := repo.Get(ctx, pool.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "主池" || got.Note != "生产密钥池" {
		t.Fatalf("Get 结果不符: %+v", got)
	}

	got.Name = "改名池"
	if err := repo.Update(ctx, pool.ID, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	again, _ := repo.Get(ctx, pool.ID)
	if again.Name != "改名池" {
		t.Fatalf("Update 未生效: %+v", again)
	}

	list, err := repo.List(ctx, PoolFilter{Name: "改名"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != pool.ID {
		t.Fatalf("List(Name 过滤) = %+v, want 命中 1 条", list)
	}

	affected, err := repo.Delete(ctx, pool.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if affected != 1 {
		t.Fatalf("Delete RowsAffected = %d, want 1", affected)
	}
	if _, err := repo.Get(ctx, pool.ID); err == nil {
		t.Fatal("删除后 Get 应报错")
	}
}

// TestCredentialRepo_List_Filters 锁定凭据按归属/状态/KeyHash 过滤。
func TestCredentialRepo_List_Filters(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolID, groupID := uint(1), uint(2)
	creds := []*models.Credential{
		{Key: "enc-1", KeyHash: "hash-a", PoolID: &poolID, Status: models.CredentialStatusActive},
		{Key: "enc-2", KeyHash: "hash-b", GroupID: &groupID, Status: models.CredentialStatusDisabled},
		{Key: "enc-3", KeyHash: "hash-c", PoolID: &poolID, Status: models.CredentialStatusError},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	byPool, err := repo.List(ctx, CredentialFilter{PoolID: &poolID})
	if err != nil {
		t.Fatalf("List byPool: %v", err)
	}
	if len(byPool) != 2 {
		t.Fatalf("byPool 数量 = %d, want 2", len(byPool))
	}

	byGroup, err := repo.List(ctx, CredentialFilter{GroupID: &groupID})
	if err != nil {
		t.Fatalf("List byGroup: %v", err)
	}
	if len(byGroup) != 1 || byGroup[0].KeyHash != "hash-b" {
		t.Fatalf("byGroup 结果不符: %+v", byGroup)
	}

	byStatus, err := repo.List(ctx, CredentialFilter{Status: models.CredentialStatusError})
	if err != nil {
		t.Fatalf("List byStatus: %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].KeyHash != "hash-c" {
		t.Fatalf("byStatus 结果不符: %+v", byStatus)
	}

	byHash, err := repo.List(ctx, CredentialFilter{KeyHash: "hash-a"})
	if err != nil {
		t.Fatalf("List byHash: %v", err)
	}
	if len(byHash) != 1 {
		t.Fatalf("byHash 数量 = %d, want 1", len(byHash))
	}

	// 三态字段清除：struct Update 会跳过零值，置空 PoolID 必须走 UpdateFields（map 显式写 NULL）
	affected, err := repo.UpdateFields(ctx, byPool[0].ID, map[string]any{"pool_id": nil})
	if err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}
	if affected != 1 {
		t.Fatalf("UpdateFields RowsAffected = %d, want 1", affected)
	}
	reloaded, _ := repo.Get(ctx, byPool[0].ID)
	if reloaded.PoolID != nil {
		t.Fatalf("置 nil 后 PoolID = %v, want nil", reloaded.PoolID)
	}
}

// TestEndpointRepo_ListByProvider 锁定端点按供应商查询与 CRUD。
func TestEndpointRepo_ListByProvider(t *testing.T) {
	ctx := context.Background()
	repo := NewEndpointRepo(newChannelTestDB(t))

	e1 := &models.Endpoint{ProviderID: 1, Protocol: "openai", URL: "", Enabled: true}
	e2 := &models.Endpoint{ProviderID: 1, Protocol: "responses", URL: "https://x.example.com", Enabled: false}
	e3 := &models.Endpoint{ProviderID: 2, Protocol: "anthropic", Enabled: true}
	for _, e := range []*models.Endpoint{e1, e2, e3} {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	list, err := repo.ListByProvider(ctx, 1)
	if err != nil {
		t.Fatalf("ListByProvider: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByProvider(1) 数量 = %d, want 2", len(list))
	}

	e2.Enabled = true
	if err := repo.Update(ctx, e2.ID, e2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.Get(ctx, e2.ID)
	if !got.Enabled {
		t.Fatal("Update 后 Enabled 应为 true")
	}

	if affected, err := repo.Delete(ctx, e3.ID); err != nil || affected != 1 {
		t.Fatalf("Delete = (%d, %v), want (1, nil)", affected, err)
	}
}

// TestKeyGroupRepo_CRUD 锁定分组 CRUD。
func TestKeyGroupRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewKeyGroupRepo(newChannelTestDB(t))

	group := &models.KeyGroup{ProviderID: 7, Name: "低价组", Weight: 3, Models: "gpt-4o-mini"}
	if err := repo.Create(ctx, group); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := repo.ListByProvider(ctx, 7)
	if err != nil {
		t.Fatalf("ListByProvider: %v", err)
	}
	if len(list) != 1 || list[0].Name != "低价组" || list[0].Weight != 3 {
		t.Fatalf("ListByProvider 结果不符: %+v", list)
	}

	poolID := uint(9)
	group.PoolID = &poolID
	if err := repo.Update(ctx, group.ID, group); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.Get(ctx, group.ID)
	if got.PoolID == nil || *got.PoolID != poolID {
		t.Fatalf("Update 后 PoolID = %v, want %d", got.PoolID, poolID)
	}

	// 三态字段清除走 UpdateFields（struct Update 跳过零值写不进 NULL）
	if _, err := repo.UpdateFields(ctx, group.ID, map[string]any{"pool_id": nil}); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}
	reloaded, _ := repo.Get(ctx, group.ID)
	if reloaded.PoolID != nil {
		t.Fatalf("置 nil 后 PoolID = %v, want nil", reloaded.PoolID)
	}
}
