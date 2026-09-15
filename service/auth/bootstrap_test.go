package auth

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

func newBootstrapEnv(t *testing.T) (*repository.Repositories, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hexKey := hex.EncodeToString([]byte("0123456789012345678901234567890a"))
	cipher, err := credentialcrypto.New(hexKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	prev := credentialcrypto.Default()
	credentialcrypto.SetDefault(cipher)
	t.Cleanup(func() { credentialcrypto.SetDefault(prev) })
	return repository.New(db), db
}

// TestBootstrapAdmin_CreatesDefaultApiKey 锁 AC-6：全新库首启建 admin 的同时
// 建一把名为 default 的调用 key（密文+hash 入库、明文返回供打印）；重启幂等。
func TestBootstrapAdmin_CreatesDefaultApiKey(t *testing.T) {
	ctx := context.Background()
	repos, db := newBootstrapEnv(t)

	res, err := BootstrapAdmin(ctx, repos, "bootstrap-pw")
	if err != nil {
		t.Fatalf("BootstrapAdmin: %v", err)
	}
	if res.Password != "bootstrap-pw" {
		t.Fatalf("Password = %q", res.Password)
	}
	if !strings.HasPrefix(res.APIKey, "sk-llmux-") {
		t.Fatalf("default key 明文不合规: %q", res.APIKey)
	}

	cipher := credentialcrypto.Default()
	var stored []models.ApiKey
	if err := db.Find(&stored).Error; err != nil {
		t.Fatalf("find keys: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("api_keys 行数 = %d, want 1", len(stored))
	}
	row := stored[0]
	if row.Name != "default" || !row.Enabled {
		t.Fatalf("default key 行不符: %+v", row)
	}
	if row.KeyHash != cipher.Hash(res.APIKey) {
		t.Fatal("KeyHash 与返回明文不自洽")
	}
	if plain, err := cipher.Decrypt(row.Key); err != nil || plain != res.APIKey {
		t.Fatalf("密文不可还原: %v", err)
	}
	var user models.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("find user: %v", err)
	}
	if row.OwnerID != user.ID {
		t.Fatalf("OwnerID = %d, want admin user %d", row.OwnerID, user.ID)
	}

	// 幂等：已有用户时重启不再建 key
	res2, err := BootstrapAdmin(ctx, repos, "another-pw")
	if err != nil {
		t.Fatalf("第二次 BootstrapAdmin: %v", err)
	}
	if res2 != (BootstrapResult{}) {
		t.Fatalf("已有用户时应返回零值, 实得 %+v", res2)
	}
	var count int64
	if err := db.Model(&models.ApiKey{}).Count(&count).Error; err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if count != 1 {
		t.Fatalf("重启后 api_keys 行数 = %d, want 恒为 1", count)
	}
}

// TestBootstrapAdmin_FailsWithoutCipher 锁装配缺失守卫：cipher 未配置时不得
// 建出「有 admin 无可用 key」的半套状态（users 与 default key 同事务进退）。
func TestBootstrapAdmin_FailsWithoutCipher(t *testing.T) {
	ctx := context.Background()
	repos, db := newBootstrapEnv(t)
	credentialcrypto.SetDefault(nil)

	if _, err := BootstrapAdmin(ctx, repos, "pw"); err == nil {
		t.Fatal("cipher 缺失应报错")
	}
	var users int64
	if err := db.Model(&models.User{}).Count(&users).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if users != 0 {
		t.Fatalf("cipher 缺失时不应留下 admin 用户（半套状态），实得 %d 行", users)
	}
}
