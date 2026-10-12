package repository

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.New(t, &models.Pool{}, &models.Credential{}, &models.Endpoint{}, &models.KeyGroup{})
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

	// struct Update 跳过零值字段：清空 Note 必须走 UpdateFields（map 显式写空串）
	if _, err := repo.UpdateFields(ctx, pool.ID, map[string]any{"note": ""}); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}
	cleared, _ := repo.Get(ctx, pool.ID)
	if cleared.Note != "" {
		t.Fatalf("UpdateFields 清空 Note 后 = %q, want 空串", cleared.Note)
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

// TestCredentialRepo_List_Filters 锁定凭据按归属号池/状态/KeyHash 过滤。
func TestCredentialRepo_List_Filters(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolA, poolB := uint(1), uint(2)
	creds := []*models.Credential{
		{Key: "enc-1", KeyHash: "hash-a", PoolID: poolA, Status: models.CredentialStatusActive},
		{Key: "enc-2", KeyHash: "hash-b", PoolID: poolB, Status: models.CredentialStatusDisabled},
		{Key: "enc-3", KeyHash: "hash-c", PoolID: poolA, Status: models.CredentialStatusError},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	byPool, err := repo.List(ctx, CredentialFilter{PoolID: &poolA})
	if err != nil {
		t.Fatalf("List byPool: %v", err)
	}
	if len(byPool) != 2 {
		t.Fatalf("byPool 数量 = %d, want 2", len(byPool))
	}

	byOtherPool, err := repo.List(ctx, CredentialFilter{PoolID: &poolB})
	if err != nil {
		t.Fatalf("List byOtherPool: %v", err)
	}
	if len(byOtherPool) != 1 || byOtherPool[0].KeyHash != "hash-b" {
		t.Fatalf("byOtherPool 结果不符: %+v", byOtherPool)
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

	// 归属改池走 UpdateFields（map 显式写值，绕开 struct Updates 跳过零值的语义）
	affected, err := repo.UpdateFields(ctx, byPool[0].ID, map[string]any{"pool_id": poolB})
	if err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}
	if affected != 1 {
		t.Fatalf("UpdateFields RowsAffected = %d, want 1", affected)
	}
	reloaded, _ := repo.Get(ctx, byPool[0].ID)
	if reloaded.PoolID != poolB {
		t.Fatalf("改池后 PoolID = %d, want %d", reloaded.PoolID, poolB)
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

// TestKeyGroupRepo_DeleteIsHard 钉分组删除是硬删（原先只打 deleted_at 标记）。
// 判据打在 Unscoped 物理行数上——软删行的不可见正是它的伪装，普通计数改前改后都是 0。
func TestKeyGroupRepo_DeleteIsHard(t *testing.T) {
	ctx := context.Background()
	db := newChannelTestDB(t)
	repo := NewKeyGroupRepo(db)

	group := &models.KeyGroup{ProviderID: 7, Name: "低价组"}
	if err := repo.Create(ctx, group); err != nil {
		t.Fatalf("Create: %v", err)
	}

	affected, err := repo.Delete(ctx, group.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if affected != 1 {
		t.Errorf("受影响行数 = %d, want 1", affected)
	}

	var leftover int64
	if err := db.Unscoped().Model(&models.KeyGroup{}).Where("id = ?", group.ID).Count(&leftover).Error; err != nil {
		t.Fatalf("unscoped count: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("删除后物理行数 = %d, want 0（仍为 1 = 走了软删）", leftover)
	}
}

// TestEndpointRepo_DeleteIsHard 钉端点删除是硬删。理由同 TestKeyGroupRepo_DeleteIsHard。
func TestEndpointRepo_DeleteIsHard(t *testing.T) {
	ctx := context.Background()
	db := newChannelTestDB(t)
	repo := NewEndpointRepo(db)

	endpoint := &models.Endpoint{ProviderID: 7, Protocol: "openai", Enabled: true}
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("Create: %v", err)
	}

	affected, err := repo.Delete(ctx, endpoint.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if affected != 1 {
		t.Errorf("受影响行数 = %d, want 1", affected)
	}

	var leftover int64
	if err := db.Unscoped().Model(&models.Endpoint{}).Where("id = ?", endpoint.ID).Count(&leftover).Error; err != nil {
		t.Fatalf("unscoped count: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("删除后物理行数 = %d, want 0（仍为 1 = 走了软删）", leftover)
	}
}

// TestEndpointRepo_CountByProviderIDs 锁定按供应商聚合端点计数（列表徽标防 N+1）。
func TestEndpointRepo_CountByProviderIDs(t *testing.T) {
	ctx := context.Background()
	repo := NewEndpointRepo(newChannelTestDB(t))

	for _, e := range []*models.Endpoint{
		{ProviderID: 1, Protocol: "openai", Enabled: true},
		{ProviderID: 1, Protocol: "responses", Enabled: true},
		{ProviderID: 2, Protocol: "anthropic", Enabled: true},
	} {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	counts, err := repo.CountByProviderIDs(ctx, []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("CountByProviderIDs: %v", err)
	}
	if counts[1] != 2 || counts[2] != 1 {
		t.Fatalf("counts = %+v, want {1:2, 2:1}", counts)
	}
	if _, ok := counts[3]; ok {
		t.Fatalf("无端点供应商 3 不应产生条目, counts=%+v", counts)
	}

	empty, err := repo.CountByProviderIDs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("空 IDs = (%v, %v), want (空 map, nil)", empty, err)
	}
}

// TestKeyGroupRepo_CountByProviderIDs 锁定按供应商聚合分组计数（列表徽标防 N+1）。
func TestKeyGroupRepo_CountByProviderIDs(t *testing.T) {
	ctx := context.Background()
	repo := NewKeyGroupRepo(newChannelTestDB(t))

	for _, g := range []*models.KeyGroup{
		{ProviderID: 10, Name: "a", Weight: 1},
		{ProviderID: 10, Name: "b", Weight: 2},
		{ProviderID: 10, Name: "c", Weight: 3},
		{ProviderID: 20, Name: "d", Weight: 1},
	} {
		if err := repo.Create(ctx, g); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	counts, err := repo.CountByProviderIDs(ctx, []uint{10, 20, 30})
	if err != nil {
		t.Fatalf("CountByProviderIDs: %v", err)
	}
	if counts[10] != 3 || counts[20] != 1 {
		t.Fatalf("counts = %+v, want {10:3, 20:1}", counts)
	}
	if _, ok := counts[30]; ok {
		t.Fatalf("无分组供应商 30 不应产生条目, counts=%+v", counts)
	}
}

// TestPoolRepo_Stats 锁定号池凭据统计聚合：一条 GROUP BY 查询返回
// key 总数与各状态计数（号池列表健康概览用，防 N+1）。
func TestPoolRepo_Stats(t *testing.T) {
	ctx := context.Background()
	db := newChannelTestDB(t)
	poolRepo := NewPoolRepo(db)
	credRepo := NewCredentialRepo(db)

	p1 := &models.Pool{Name: "主池"}
	p2 := &models.Pool{Name: "备用池"}
	for _, p := range []*models.Pool{p1, p2} {
		if err := poolRepo.Create(ctx, p); err != nil {
			t.Fatalf("Create pool: %v", err)
		}
	}

	creds := []*models.Credential{
		{Key: "enc-1", KeyHash: "h-a", PoolID: p1.ID, Status: models.CredentialStatusActive},
		{Key: "enc-2", KeyHash: "h-b", PoolID: p1.ID, Status: models.CredentialStatusActive},
		{Key: "enc-3", KeyHash: "h-c", PoolID: p1.ID, Status: models.CredentialStatusError},
		{Key: "enc-4", KeyHash: "h-d", PoolID: p2.ID, Status: models.CredentialStatusDisabled},
	}
	for _, c := range creds {
		if err := credRepo.Create(ctx, c); err != nil {
			t.Fatalf("Create credential: %v", err)
		}
	}

	stats, err := poolRepo.StatsByIDs(ctx, []uint{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("StatsByIDs: %v", err)
	}

	s1, ok := stats[p1.ID]
	if !ok {
		t.Fatalf("stats 缺 p1=%d 条目: %+v", p1.ID, stats)
	}
	if s1.KeyCount != 3 {
		t.Fatalf("p1 KeyCount = %d, want 3", s1.KeyCount)
	}
	if s1.StatusCount[models.CredentialStatusActive] != 2 || s1.StatusCount[models.CredentialStatusError] != 1 {
		t.Fatalf("p1 StatusCount 不符: %+v", s1.StatusCount)
	}

	s2, ok := stats[p2.ID]
	if !ok {
		t.Fatalf("stats 缺 p2=%d 条目: %+v", p2.ID, stats)
	}
	if s2.KeyCount != 1 || s2.StatusCount[models.CredentialStatusDisabled] != 1 {
		t.Fatalf("p2 统计不符: %+v", s2)
	}

	// 无凭据的号池不产生条目（调用方初始化为零值）
	p3 := &models.Pool{Name: "空池"}
	if err := poolRepo.Create(ctx, p3); err != nil {
		t.Fatalf("Create empty pool: %v", err)
	}
	emptyStats, err := poolRepo.StatsByIDs(ctx, []uint{p3.ID})
	if err != nil {
		t.Fatalf("StatsByIDs(empty): %v", err)
	}
	if len(emptyStats) != 0 {
		t.Fatalf("空池应无统计条目, got %d", len(emptyStats))
	}

	// ids 为空返回空 map（handler 空列表时调用）
	nilStats, err := poolRepo.StatsByIDs(ctx, nil)
	if err != nil {
		t.Fatalf("StatsByIDs(nil): %v", err)
	}
	if len(nilStats) != 0 {
		t.Fatalf("StatsByIDs(nil) 应返回空 map, got %d", len(nilStats))
	}
}

// TestCredentialRepo_DeleteByPool 锁定按号池级联删除凭据（软删）；
// 归属其他号池的凭据不受影响（凭据只归属号池，无内联归属例外）。
func TestCredentialRepo_DeleteByPool(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolA, poolB := uint(1), uint(2)
	creds := []*models.Credential{
		{Key: "enc-1", KeyHash: "h1", PoolID: poolA},
		{Key: "enc-2", KeyHash: "h2", PoolID: poolA},
		{Key: "enc-3", KeyHash: "h3", PoolID: poolB},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	affected, err := repo.DeleteByPoolID(ctx, poolA)
	if err != nil {
		t.Fatalf("DeleteByPoolID: %v", err)
	}
	if affected != 2 {
		t.Fatalf("DeleteByPoolID RowsAffected = %d, want 2", affected)
	}

	byPool, err := repo.List(ctx, CredentialFilter{PoolID: &poolA})
	if err != nil {
		t.Fatalf("List byPool: %v", err)
	}
	if len(byPool) != 0 {
		t.Fatalf("级联删除后池内凭据数 = %d, want 0", len(byPool))
	}

	byOtherPool, err := repo.List(ctx, CredentialFilter{PoolID: &poolB})
	if err != nil {
		t.Fatalf("List byOtherPool: %v", err)
	}
	if len(byOtherPool) != 1 || byOtherPool[0].KeyHash != "h3" {
		t.Fatalf("他池凭据不应被级联删除: %+v", byOtherPool)
	}
}

// TestKeyGroupRepo_CountByPoolIDs 锁定号池被分组引用计数
// （DELETE /api/pools/:id 引用守卫的依据）。
func TestKeyGroupRepo_CountByPoolIDs(t *testing.T) {
	ctx := context.Background()
	repo := NewKeyGroupRepo(newChannelTestDB(t))

	p1, p2, p3 := uint(1), uint(2), uint(3)
	groups := []*models.KeyGroup{
		{ProviderID: 1, Name: "默认组", PoolID: &p1},
		{ProviderID: 1, Name: "低价组", PoolID: &p1},
		{ProviderID: 2, Name: "走量组", PoolID: &p2},
		{ProviderID: 3, Name: "无池组"}, // 未引用号池
	}
	for _, g := range groups {
		if err := repo.Create(ctx, g); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	counts, err := repo.CountByPoolIDs(ctx, []uint{p1, p2, p3})
	if err != nil {
		t.Fatalf("CountByPoolIDs: %v", err)
	}
	if counts[p1] != 2 {
		t.Fatalf("p1 引用数 = %d, want 2", counts[p1])
	}
	if counts[p2] != 1 {
		t.Fatalf("p2 引用数 = %d, want 1", counts[p2])
	}
	if counts[p3] != 0 {
		t.Fatalf("p3 引用数 = %d, want 0", counts[p3])
	}

	empty, err := repo.CountByPoolIDs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("CountByPoolIDs(nil) = (%v, %v), want (empty, nil)", empty, err)
	}
}

// TestCredentialRepo_BatchByIDs 锁定按 ID 批量启停/删除语义：凭据是第一等资源后，
// 批量口径 = 给定 IDs 命中即改，不再有号池限界（归属过滤只存在于列表查询侧）——
// 归属不同号池的凭据在同一次调用里一并生效。
func TestCredentialRepo_BatchByIDs(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolA, poolB, poolC := uint(1), uint(2), uint(3)
	creds := []*models.Credential{
		{Key: "enc-a1", KeyHash: "ha1", PoolID: poolA, Status: models.CredentialStatusActive},
		{Key: "enc-a2", KeyHash: "ha2", PoolID: poolA, Status: models.CredentialStatusActive},
		{Key: "enc-b1", KeyHash: "hb1", PoolID: poolB, Status: models.CredentialStatusActive},
		{Key: "enc-c1", KeyHash: "hc1", PoolID: poolC, Status: models.CredentialStatusActive},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	all := []uint{creds[0].ID, creds[1].ID, creds[2].ID, creds[3].ID}

	// 批量启停：跨号池的 ID 一律命中
	statusFields := map[string]any{"status": models.CredentialStatusDisabled}
	updated, err := repo.UpdateFieldsByIDs(ctx, all, statusFields)
	if err != nil {
		t.Fatalf("UpdateFieldsByIDs: %v", err)
	}
	if updated != 4 {
		t.Fatalf("UpdateFieldsByIDs RowsAffected = %d, want 4", updated)
	}
	for _, c := range creds {
		got, err := repo.Get(ctx, c.ID)
		if err != nil {
			t.Fatalf("Get(%d): %v", c.ID, err)
		}
		if got.Status != models.CredentialStatusDisabled {
			t.Fatalf("凭据 %d 未更新: %+v", c.ID, got)
		}
	}

	// 空 IDs 直接返回 0（不产生 SQL）
	if updated, err := repo.UpdateFieldsByIDs(ctx, nil, statusFields); err != nil || updated != 0 {
		t.Fatalf("UpdateFieldsByIDs(nil) = (%d, %v), want (0, nil)", updated, err)
	}

	// 批量删除：同样按 ID 命中，跨号池一并软删
	deleted, err := repo.DeleteByIDs(ctx, all)
	if err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}
	if deleted != 4 {
		t.Fatalf("DeleteByIDs RowsAffected = %d, want 4", deleted)
	}
	byPoolA, err := repo.List(ctx, CredentialFilter{PoolID: &poolA})
	if err != nil {
		t.Fatalf("List byPoolA: %v", err)
	}
	if len(byPoolA) != 0 {
		t.Fatalf("池A剩余凭据数 = %d, want 0（已软删）", len(byPoolA))
	}
	byPoolC, err := repo.List(ctx, CredentialFilter{PoolID: &poolC})
	if err != nil {
		t.Fatalf("List byPoolC: %v", err)
	}
	if len(byPoolC) != 0 {
		t.Fatalf("池C剩余凭据数 = %d, want 0（已软删）", len(byPoolC))
	}
}

// TestCredentialRepo_ListByPools 锁定装配侧收敛查询语义：
//
//	命中 = pool_id IN poolIDs；结果按 id ASC（组内轮询取模基线）；
//	poolIDs 为空返回 nil 且不产生 SQL。
func TestCredentialRepo_ListByPools(t *testing.T) {
	ctx := context.Background()
	db := newChannelTestDB(t)
	repo := NewCredentialRepo(db)

	if err := db.Create(&[]models.Credential{
		{Key: "c1", KeyHash: "h1", PoolID: 1},
		{Key: "c2", KeyHash: "h2", PoolID: 2},
		{Key: "c3", KeyHash: "h3", PoolID: 3},
		{Key: "c-pool", KeyHash: "h-pool", PoolID: 9},
	}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	// 只命中池 1 + 2 + 9：c1 / c2 / c-pool
	got, err := repo.ListByPools(ctx, []uint{1, 2, 9})
	if err != nil {
		t.Fatalf("ListByPools: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListByPools = %d 条, want 3（池1/池2/池9）", len(got))
	}
	if got[0].KeyHash != "h1" || got[1].KeyHash != "h2" || got[2].KeyHash != "h-pool" {
		t.Fatalf("ListByPools 顺序 = [%s %s %s], want [h1 h2 h-pool]（id ASC）", got[0].KeyHash, got[1].KeyHash, got[2].KeyHash)
	}

	// 无命中池 → 空
	empty, err := repo.ListByPools(ctx, []uint{99})
	if err != nil {
		t.Fatalf("ListByPools(99): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ListByPools(99) = %d 条, want 0", len(empty))
	}

	// 空 poolIDs → nil 不产生 SQL
	noop, err := repo.ListByPools(ctx, nil)
	if err != nil {
		t.Fatalf("ListByPools(nil): %v", err)
	}
	if noop != nil {
		t.Fatalf("ListByPools(nil) = %v, want nil", noop)
	}
}

// TestCredentialRepo_ExistingHashes 锁定批量导入查重的仓储语义：
// 命中/未命中、池内限界（跨池同 hash 不计）、软删行排除（语义 =「现存未删行去重」）、
// 空 hashes 不产生查询。
func TestCredentialRepo_ExistingHashes(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolA, poolB, poolC := uint(1), uint(2), uint(3)
	creds := []*models.Credential{
		{Key: "enc-1", KeyHash: "hash-keep", PoolID: poolA},
		{Key: "enc-2", KeyHash: "hash-deleted", PoolID: poolA},
		{Key: "enc-3", KeyHash: "hash-poolb", PoolID: poolB},
		{Key: "enc-4", KeyHash: "hash-poolc", PoolID: poolC},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	// 软删一条，验证查询自动排除
	if _, err := repo.Delete(ctx, creds[1].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// 池A：命中现存行，未命中不返回；池B/池C 与软删行不算池A命中
	got, err := repo.ExistingHashes(ctx, poolA, []string{"hash-keep", "hash-deleted", "hash-poolb", "hash-poolc", "hash-none"})
	if err != nil {
		t.Fatalf("ExistingHashes: %v", err)
	}
	if !got["hash-keep"] {
		t.Fatalf("hash-keep 应命中: %+v", got)
	}
	if got["hash-deleted"] {
		t.Fatalf("软删行 hash-deleted 不应命中: %+v", got)
	}
	if got["hash-poolb"] {
		t.Fatalf("跨池 hash-poolb 不应命中池A: %+v", got)
	}
	if got["hash-poolc"] {
		t.Fatalf("跨池 hash-poolc 不应命中池A: %+v", got)
	}
	if got["hash-none"] {
		t.Fatalf("不存在 hash-none 不应命中: %+v", got)
	}
	if len(got) != 1 {
		t.Fatalf("命中数 = %d, want 1: %+v", len(got), got)
	}

	// 池B 命中自己的行
	gotB, err := repo.ExistingHashes(ctx, poolB, []string{"hash-poolb", "hash-keep"})
	if err != nil {
		t.Fatalf("ExistingHashes(poolB): %v", err)
	}
	if !gotB["hash-poolb"] || gotB["hash-keep"] {
		t.Fatalf("池B查重结果不符: %+v", gotB)
	}

	// 空 hashes 直接返回空 map（不产生 SQL）
	empty, err := repo.ExistingHashes(ctx, poolA, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("ExistingHashes(nil) = (%v, %v), want (empty, nil)", empty, err)
	}
}

// TestCredentialRepo_UpdateFieldsIfStatus 锁探活恢复条件更新（#6-3）：
// status 匹配才写；不匹配 → RowsAffected=0 且行不变。
func TestCredentialRepo_UpdateFieldsIfStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))
	cred := &models.Credential{
		Key: "enc", KeyHash: "h", Status: models.CredentialStatusTempUnsched, FailCount: 3,
	}
	if err := repo.Create(ctx, cred); err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := repo.UpdateFieldsIfStatus(ctx, cred.ID, models.CredentialStatusTempUnsched, map[string]any{
		"status": models.CredentialStatusActive, "fail_count": 0,
	})
	if err != nil || n != 1 {
		t.Fatalf("match update = (%d, %v), want (1, nil)", n, err)
	}
	got, _ := repo.Get(ctx, cred.ID)
	if got.Status != models.CredentialStatusActive || got.FailCount != 0 {
		t.Fatalf("after match: %+v", got)
	}

	n, err = repo.UpdateFieldsIfStatus(ctx, cred.ID, models.CredentialStatusTempUnsched, map[string]any{
		"status": models.CredentialStatusDisabled,
	})
	if err != nil || n != 0 {
		t.Fatalf("mismatch update = (%d, %v), want (0, nil)", n, err)
	}
	got, _ = repo.Get(ctx, cred.ID)
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("mismatch must not overwrite: status=%q", got.Status)
	}
}
