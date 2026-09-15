package repository

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newApiKeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.ApiKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newApiKeyTestCipher 固定密钥的可注入 Cipher（测试不碰包级 Default 全局态）。
func newApiKeyTestCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	// 32 字节 hex 任意固定值即可，仅测试内部自洽（加密-解密-哈希同源）。
	hexKey := hex.EncodeToString([]byte("0123456789012345678901234567890a"))
	cipher, err := credentialcrypto.New(hexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	return cipher
}

func newStoredApiKey(t *testing.T, cipher *credentialcrypto.Cipher, ownerID uint, name, plain string) *models.ApiKey {
	t.Helper()
	encrypted, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return &models.ApiKey{
		OwnerID: ownerID,
		Name:    name,
		Key:     encrypted,
		KeyHash: cipher.Hash(plain),
		Enabled: true,
	}
}

// TestApiKeyRepo_CreateStoresCipherAndHash 锁 AC-1 存储形状：
// 表内不存明文、密文可还原、hash 为 SHA-256(明文) 且可按 hash 点查命中。
func TestApiKeyRepo_CreateStoresCipherAndHash(t *testing.T) {
	ctx := context.Background()
	repo := NewApiKeyRepo(newApiKeyTestDB(t))
	cipher := newApiKeyTestCipher(t)

	plain := "sk-llmux-create-shape-check"
	k := newStoredApiKey(t, cipher, 1, "主 key", plain)
	if err := repo.Create(ctx, k); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if k.ID == 0 {
		t.Fatal("ID 未回填")
	}
	if k.Key == plain {
		t.Fatal("Key 列存了明文")
	}
	if got, err := cipher.Decrypt(k.Key); err != nil || got != plain {
		t.Fatalf("密文不可还原: %v %q", err, got)
	}

	got, err := repo.FindByHash(ctx, cipher.Hash(plain))
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if got.Name != "主 key" || !got.Enabled || got.OwnerID != 1 {
		t.Fatalf("FindByHash 返回行不符: %+v", got)
	}
}

// TestApiKeyRepo_CreateRejectsEmptyHash 锁空 hash 不入库：
// hash 列 NOT NULL + 唯一索引若放行空串，第一把异常行会永久占位。
func TestApiKeyRepo_CreateRejectsEmptyHash(t *testing.T) {
	ctx := context.Background()
	repo := NewApiKeyRepo(newApiKeyTestDB(t))

	err := repo.Create(ctx, &models.ApiKey{OwnerID: 1, Name: "x", Key: "whatever"})
	if err == nil {
		t.Fatal("空 KeyHash 的 Create 不应成功")
	}
}

// TestApiKeyRepo_CreateDuplicateHash 锁 AC-3：同明文二次 Create → ErrApiKeyExists
// （应用层不预查，靠 uniqueIndex 撞库翻译——方案 §5 定案）；同名不同值不受影响。
func TestApiKeyRepo_CreateDuplicateHash(t *testing.T) {
	ctx := context.Background()
	repo := NewApiKeyRepo(newApiKeyTestDB(t))
	cipher := newApiKeyTestCipher(t)

	plain := "sk-llmux-dup-collision-001"
	if err := repo.Create(ctx, newStoredApiKey(t, cipher, 1, "第一把", plain)); err != nil {
		t.Fatalf("首次 Create: %v", err)
	}

	err := repo.Create(ctx, newStoredApiKey(t, cipher, 1, "复制品", plain))
	if !errors.Is(err, ErrApiKeyExists) {
		t.Fatalf("撞库应得 ErrApiKeyExists，实得 %v", err)
	}

	// 重名允许（方案 §5：name 不唯一），不同明文不受 name 影响
	if err := repo.Create(ctx, newStoredApiKey(t, cipher, 1, "第一把", "sk-llmux-different-value")); err != nil {
		t.Fatalf("同名不同值 Create 应成功: %v", err)
	}
}

// TestApiKeyRepo_ListAndGet 锁 AC-4 读侧：List 返回全部行（含密文原样，解密归调用方）。
func TestApiKeyRepo_ListAndGet(t *testing.T) {
	ctx := context.Background()
	repo := NewApiKeyRepo(newApiKeyTestDB(t))
	cipher := newApiKeyTestCipher(t)

	first := newStoredApiKey(t, cipher, 1, "a-key", "sk-llmux-list-first-value")
	second := newStoredApiKey(t, cipher, 1, "b-key", "sk-llmux-list-second-value")
	second.Enabled = false
	for _, k := range []*models.ApiKey{first, second} {
		if err := repo.Create(ctx, k); err != nil {
			t.Fatalf("Create %s: %v", k.Name, err)
		}
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List 长度 = %d, want 2", len(list))
	}

	got, err := repo.Get(ctx, second.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Enabled {
		t.Fatal("Get 行 Enabled 应为 false")
	}
}

// TestApiKeyRepo_UpdateFields 锁 AC-5 改侧：重命名/启停走 map 更新（绕过零值跳过）。
func TestApiKeyRepo_UpdateFields(t *testing.T) {
	ctx := context.Background()
	repo := NewApiKeyRepo(newApiKeyTestDB(t))
	cipher := newApiKeyTestCipher(t)

	k := newStoredApiKey(t, cipher, 1, "旧名", "sk-llmux-update-fields-01")
	if err := repo.Create(ctx, k); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := repo.UpdateFields(ctx, k.ID, map[string]any{"name": "新名", "enabled": false})
	if err != nil || rows != 1 {
		t.Fatalf("UpdateFields: rows=%d err=%v", rows, err)
	}
	got, err := repo.Get(ctx, k.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "新名" || got.Enabled {
		t.Fatalf("更新未生效: %+v", got)
	}
	if got.Key != k.Key {
		t.Fatal("启停/改名不得触碰密文")
	}
}

// TestApiKeyRepo_DeleteUnscopesHard 锁 AC-5 删侧 + 唯一索引坑：
// 软删残留会挡住同值重建（purgeSoftDeleted 同款教训），Delete 必须硬删——
// 删除后同明文 Create 应成功。
func TestApiKeyRepo_DeleteUnscopesHard(t *testing.T) {
	ctx := context.Background()
	repo := NewApiKeyRepo(newApiKeyTestDB(t))
	cipher := newApiKeyTestCipher(t)

	plain := "sk-llmux-delete-then-reuse"
	k := newStoredApiKey(t, cipher, 1, "将删", plain)
	if err := repo.Create(ctx, k); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := repo.Delete(ctx, k.ID)
	if err != nil || rows != 1 {
		t.Fatalf("Delete: rows=%d err=%v", rows, err)
	}
	if _, err := repo.Get(ctx, k.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("删后 Get 应 RecordNotFound，实得 %v", err)
	}
	// 物理层确认无软删残留行
	var rawCount int64
	if err := repo.(*apiKeyRepo).db.Unscoped().Model(&models.ApiKey{}).Count(&rawCount).Error; err != nil {
		t.Fatalf("raw count: %v", err)
	}
	if rawCount != 0 {
		t.Fatalf("软删残留 %d 行：Delete 未硬删，同值重建会被唯一索引挡死", rawCount)
	}
	if err := repo.Create(ctx, newStoredApiKey(t, cipher, 1, "重建", plain)); err != nil {
		t.Fatalf("删后同值重建应成功: %v", err)
	}
}
