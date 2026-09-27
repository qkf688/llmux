package repository

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
)

// Stage 27 存量空明文凭据清洗的回归：启动时「解密成功但明文为空（含全空白）」的
// 凭据行被删除，「解密失败」（非密文/损坏）与正常行原样保留——空明文行已无法通过
// 选路（DecryptCredentialKey 空明文守卫），留着只会在后台显示 active 误导管理员；
// 解密失败行不删是「清洗只删可判定为空」的收敛口径（损坏行可人工排查）。
// cipher 未初始化时整体跳过不报错（启动装配序允许清洗先于密钥加载吗——不允许，
// main 装配序已保证 cipher 先就绪；此分支防御 nil panic）。
func TestDeleteLegacyEmptyKeyCredentials(t *testing.T) {
	c, err := credentialcrypto.New(testCredHexKeyRepo)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })

	encEmpty, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	encBlank, err := c.Encrypt("  \t ")
	if err != nil {
		t.Fatalf("encrypt blank: %v", err)
	}
	encOK, err := c.Encrypt("sk-keep-me-1")
	if err != nil {
		t.Fatalf("encrypt ok: %v", err)
	}

	ctx := context.Background()
	repo := NewCredentialRepo(newCredentialTestDB(t))

	emptyCred := models.Credential{Key: encEmpty, Status: models.CredentialStatusActive}
	if err := repo.Create(ctx, &emptyCred); err != nil {
		t.Fatalf("create empty: %v", err)
	}
	blankCred := models.Credential{Key: encBlank, Status: models.CredentialStatusActive}
	if err := repo.Create(ctx, &blankCred); err != nil {
		t.Fatalf("create blank: %v", err)
	}
	okCred := models.Credential{Key: encOK, KeyHash: "h-ok", Status: models.CredentialStatusActive}
	if err := repo.Create(ctx, &okCred); err != nil {
		t.Fatalf("create ok: %v", err)
	}
	// 解密失败行：非 hex 密文（存量形态 A）。
	brokenCred := models.Credential{Key: "not-hex-ciphertext", Status: models.CredentialStatusActive}
	if err := repo.Create(ctx, &brokenCred); err != nil {
		t.Fatalf("create broken: %v", err)
	}

	deleted, err := repo.DeleteLegacyEmptyKeyCredentials(ctx)
	if err != nil {
		t.Fatalf("DeleteLegacyEmptyKeyCredentials: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2（空明文 + 全空白）", deleted)
	}

	// 空明文两行已软删（Get 走未软删作用域 → 404），正常行与解密失败行原样。
	if _, err := repo.Get(ctx, emptyCred.ID); err == nil {
		t.Fatal("空明文行应已删除，仍可 Get 到")
	}
	if _, err := repo.Get(ctx, blankCred.ID); err == nil {
		t.Fatal("全空白明文行应已删除，仍可 Get 到")
	}
	kept, err := repo.Get(ctx, okCred.ID)
	if err != nil || kept.Key != encOK {
		t.Fatalf("正常行被误删/被改: err=%v key_match=%v", err, err == nil && kept.Key == encOK)
	}
	keptBroken, err := repo.Get(ctx, brokenCred.ID)
	if err != nil || keptBroken.Key != "not-hex-ciphertext" {
		t.Fatalf("解密失败行应保留: err=%v", err)
	}

	// 幂等：再跑一遍零删。
	deleted2, err := repo.DeleteLegacyEmptyKeyCredentials(ctx)
	if err != nil || deleted2 != 0 {
		t.Fatalf("second pass deleted = %d err = %v, want 0/nil（幂等）", deleted2, err)
	}
}
