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
		{Key: "enc-1", KeyHash: "h-a", PoolID: &p1.ID, Status: models.CredentialStatusActive},
		{Key: "enc-2", KeyHash: "h-b", PoolID: &p1.ID, Status: models.CredentialStatusActive},
		{Key: "enc-3", KeyHash: "h-c", PoolID: &p1.ID, Status: models.CredentialStatusError},
		{Key: "enc-4", KeyHash: "h-d", PoolID: &p2.ID, Status: models.CredentialStatusDisabled},
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
// 分组内联凭据（GroupID 归属）不受影响。
func TestCredentialRepo_DeleteByPool(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolID, groupID := uint(1), uint(2)
	creds := []*models.Credential{
		{Key: "enc-1", KeyHash: "h1", PoolID: &poolID},
		{Key: "enc-2", KeyHash: "h2", PoolID: &poolID},
		{Key: "enc-3", KeyHash: "h3", GroupID: &groupID},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	affected, err := repo.DeleteByPoolID(ctx, poolID)
	if err != nil {
		t.Fatalf("DeleteByPoolID: %v", err)
	}
	if affected != 2 {
		t.Fatalf("DeleteByPoolID RowsAffected = %d, want 2", affected)
	}

	byPool, err := repo.List(ctx, CredentialFilter{PoolID: &poolID})
	if err != nil {
		t.Fatalf("List byPool: %v", err)
	}
	if len(byPool) != 0 {
		t.Fatalf("级联删除后池内凭据数 = %d, want 0", len(byPool))
	}

	byGroup, err := repo.List(ctx, CredentialFilter{GroupID: &groupID})
	if err != nil {
		t.Fatalf("List byGroup: %v", err)
	}
	if len(byGroup) != 1 || byGroup[0].KeyHash != "h3" {
		t.Fatalf("分组内联凭据不应被级联删除: %+v", byGroup)
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
		{ProviderID: 3, Name: "内联组"}, // 无 PoolID（内联凭据）
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

// TestCredentialRepo_BatchWithinPool 锁定批量启停/删除的池内限界语义：
// 越池 ID 不计入 RowsAffected，也不改动他池数据（S2-3 批量端点防越池误操作）。
func TestCredentialRepo_BatchWithinPool(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newChannelTestDB(t))

	poolA, poolB := uint(1), uint(2)
	creds := []*models.Credential{
		{Key: "enc-a1", KeyHash: "ha1", PoolID: &poolA, Status: models.CredentialStatusActive},
		{Key: "enc-a2", KeyHash: "ha2", PoolID: &poolA, Status: models.CredentialStatusActive},
		{Key: "enc-a3", KeyHash: "ha3", PoolID: &poolA, Status: models.CredentialStatusActive},
		{Key: "enc-b1", KeyHash: "hb1", PoolID: &poolB, Status: models.CredentialStatusActive},
	}
	for _, c := range creds {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	idsA := []uint{creds[0].ID, creds[1].ID}
	idsB := []uint{creds[3].ID}

	// 批量启停：池A操作带入池B的ID，只应命中池A的2条
	updated, err := repo.UpdateStatusByIDs(ctx, poolA, append(idsA, idsB...), models.CredentialStatusDisabled)
	if err != nil {
		t.Fatalf("UpdateStatusByIDs: %v", err)
	}
	if updated != 2 {
		t.Fatalf("UpdateStatusByIDs RowsAffected = %d, want 2", updated)
	}
	got, _ := repo.Get(ctx, creds[0].ID)
	if got.Status != models.CredentialStatusDisabled {
		t.Fatalf("池A凭据未更新: %+v", got)
	}
	gotB, _ := repo.Get(ctx, creds[3].ID)
	if gotB.Status != models.CredentialStatusActive {
		t.Fatalf("池B凭据被越池改动: %+v", gotB)
	}

	// 空 IDs 直接返回 0（不产生 SQL）
	if updated, err := repo.UpdateStatusByIDs(ctx, poolA, nil, models.CredentialStatusActive); err != nil || updated != 0 {
		t.Fatalf("UpdateStatusByIDs(nil) = (%d, %v), want (0, nil)", updated, err)
	}

	// 批量删除：同样只删池内
	deleted, err := repo.DeleteByIDs(ctx, poolA, append(idsA, idsB...))
	if err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("DeleteByIDs RowsAffected = %d, want 2", deleted)
	}
	byPoolA, err := repo.List(ctx, CredentialFilter{PoolID: &poolA})
	if err != nil {
		t.Fatalf("List byPoolA: %v", err)
	}
	if len(byPoolA) != 1 {
		t.Fatalf("池A剩余凭据数 = %d, want 1", len(byPoolA))
	}
	byPoolB, _ := repo.List(ctx, CredentialFilter{PoolID: &poolB})
	if len(byPoolB) != 1 {
		t.Fatalf("池B凭据数 = %d, want 1（越池不应被删）", len(byPoolB))
	}
}
