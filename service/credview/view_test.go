package credview

import (
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
)

const testHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(testHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return c
}

// TestViewOf_MaskAndNilSemantics 锁视图映射的两条契约：
// 掩码走 MaskStored 单源；无归属/无冷却的时间与指针字段保持 nil（序列化成 JSON null，
// 前端据此区分「无」与「值」，故 View 上这些字段禁 omitempty）。
func TestViewOf_MaskAndNilSemantics(t *testing.T) {
	cipher := setupCipher(t)
	plain := "sk-view-ABCD1234"
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	cred := models.Credential{
		Key:            enc,
		Status:         models.CredentialStatusTempUnsched,
		FailCount:      3,
		CooldownReason: "auth_fail",
	}
	cred.ID = 7

	view := ViewOf(cred)
	if view.ID != 7 {
		t.Fatalf("ID=%d want 7", view.ID)
	}
	if want := credentialcrypto.MaskKey(plain); view.KeyMasked != want {
		t.Fatalf("KeyMasked=%q want %q", view.KeyMasked, want)
	}
	if view.Status != models.CredentialStatusTempUnsched || view.FailCount != 3 || view.CooldownReason != "auth_fail" {
		t.Fatalf("状态字段未如实透出: %+v", view)
	}
	if view.PoolID != nil || view.GroupID != nil || view.CooldownUntil != nil ||
		view.LastUsedAt != nil || view.LastProbeAt != nil {
		t.Fatalf("无归属/无冷却的指针字段应为 nil: %+v", view)
	}
}

// TestViewOf_NoCipher_DegradesToStars cipher 未配置时一律 `****`——绝不输出密文片段。
func TestViewOf_NoCipher_DegradesToStars(t *testing.T) {
	credentialcrypto.SetDefault(nil)
	view := ViewOf(models.Credential{Key: "deadbeefdeadbeef", Status: models.CredentialStatusActive})
	if view.KeyMasked != "****" {
		t.Fatalf("KeyMasked=%q want ****（cipher 未配置的降级）", view.KeyMasked)
	}
}

// TestViewsOf_EmptyIsEmptySlice 空输入回非 nil 空切片：JSON 序列化成 `[]` 而非 null，
// 前端列表不必写判空分支。
func TestViewsOf_EmptyIsEmptySlice(t *testing.T) {
	views := ViewsOf(nil)
	if views == nil || len(views) != 0 {
		t.Fatalf("ViewsOf(nil) = %#v, want 非 nil 空切片", views)
	}
}
