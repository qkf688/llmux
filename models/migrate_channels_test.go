package models

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

func testChannelDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&Provider{}, &Pool{}, &Credential{}, &Endpoint{}, &KeyGroup{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testChannelCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	return c
}

// TestMigrateLegacyProviders_ExpandsLegacyShape 锁定迁移展开形状：
// 1 端点 + 1 分组 + 1 内联凭据（密文 + KeyHash），Config.api_key 清空、base_url 保留，
// Protocols 按 type 派生。
func TestMigrateLegacyProviders_ExpandsLegacyShape(t *testing.T) {
	db := testChannelDB(t)
	legacy := &Provider{
		Name:   "legacy-openai",
		Type:   "openai",
		Config: `{"api_key":"sk-legacy-123","base_url":"https://api.example.com/v1"}`,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	cipher := testChannelCipher(t)
	migrateLegacyProviders(context.Background(), db, cipher)

	// Protocols 派生
	var got Provider
	if err := db.First(&got, legacy.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if len(got.Protocols) != 1 || got.Protocols[0] != string(consts.ProtocolOpenAI) {
		t.Fatalf("Protocols = %v, want [openai]", got.Protocols)
	}

	// 1 端点
	var endpoints []Endpoint
	if err := db.Where("provider_id = ?", legacy.ID).Find(&endpoints).Error; err != nil {
		t.Fatalf("list endpoints: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("endpoints 数量 = %d, want 1", len(endpoints))
	}
	if endpoints[0].Protocol != string(consts.ProtocolOpenAI) || endpoints[0].URL != "" || !endpoints[0].Enabled {
		t.Fatalf("endpoint 形状不符: %+v", endpoints[0])
	}

	// 1 分组 + 1 凭据（挂在分组下）
	var groups []KeyGroup
	if err := db.Where("provider_id = ?", legacy.ID).Find(&groups).Error; err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "默认组" || groups[0].Weight != 1 {
		t.Fatalf("groups 形状不符: %+v", groups)
	}

	var creds []Credential
	if err := db.Where("key_hash = ?", cipher.Hash("sk-legacy-123")).Find(&creds).Error; err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("credentials 数量 = %d, want 1", len(creds))
	}
	cred := creds[0]
	if cred.GroupID == nil || *cred.GroupID != groups[0].ID {
		t.Fatalf("credential.GroupID = %v, want 分组 %d", cred.GroupID, groups[0].ID)
	}
	if cred.Status != CredentialStatusActive {
		t.Fatalf("credential.Status = %q, want active", cred.Status)
	}
	plain, err := cipher.Decrypt(cred.Key)
	if err != nil {
		t.Fatalf("解密凭据失败（密文不可读）: %v", err)
	}
	if plain != "sk-legacy-123" {
		t.Fatalf("解密 = %q, want sk-legacy-123", plain)
	}
	if cred.Key == plain {
		t.Fatal("凭据 Key 落库为明文")
	}

	// Config：api_key 清空、base_url 保留
	if gjson.Get(got.Config, "api_key").Exists() {
		t.Fatalf("Config.api_key 未清空: %s", got.Config)
	}
	if got := gjson.Get(got.Config, "base_url").String(); got != "https://api.example.com/v1" {
		t.Fatalf("Config.base_url = %q, 应保留", got)
	}
}

// TestMigrateLegacyProviders_Idempotent 锁定幂等：二次迁移不重复生成任何行。
func TestMigrateLegacyProviders_Idempotent(t *testing.T) {
	db := testChannelDB(t)
	legacy := &Provider{
		Name:   "legacy-openai",
		Type:   "openai",
		Config: `{"api_key":"sk-legacy-123","base_url":"https://api.example.com/v1"}`,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	cipher := testChannelCipher(t)
	migrateLegacyProviders(context.Background(), db, cipher)
	migrateLegacyProviders(context.Background(), db, cipher)

	var endpointCount, groupCount, credCount int64
	db.Model(&Endpoint{}).Where("provider_id = ?", legacy.ID).Count(&endpointCount)
	db.Model(&KeyGroup{}).Where("provider_id = ?", legacy.ID).Count(&groupCount)
	db.Model(&Credential{}).Where("key_hash = ?", cipher.Hash("sk-legacy-123")).Count(&credCount)
	if endpointCount != 1 || groupCount != 1 || credCount != 1 {
		t.Fatalf("二次迁移后 endpoint=%d group=%d credential=%d, want 1/1/1", endpointCount, groupCount, credCount)
	}
}

// TestMigrateLegacyProviders_ProtocolsDerived 锁定 type→协议映射在迁移里生效
// （openai-res → responses，anthropic → anthropic）。
func TestMigrateLegacyProviders_ProtocolsDerived(t *testing.T) {
	cipher := testChannelCipher(t)

	tests := []struct {
		name         string
		typ          string
		wantProtocol string
	}{
		{name: "openai", typ: "openai", wantProtocol: string(consts.ProtocolOpenAI)},
		{name: "openai-res", typ: "openai-res", wantProtocol: string(consts.ProtocolResponses)},
		{name: "anthropic", typ: "anthropic", wantProtocol: string(consts.ProtocolAnthropic)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testChannelDB(t)
			p := &Provider{Name: "p-" + tt.typ, Type: tt.typ, Config: `{"api_key":"sk-x"}`}
			if err := db.Create(p).Error; err != nil {
				t.Fatalf("create provider: %v", err)
			}
			migrateLegacyProviders(context.Background(), db, cipher)

			var got Provider
			if err := db.First(&got, p.ID).Error; err != nil {
				t.Fatalf("reload provider: %v", err)
			}
			if len(got.Protocols) != 1 || got.Protocols[0] != tt.wantProtocol {
				t.Fatalf("Protocols = %v, want [%s]", got.Protocols, tt.wantProtocol)
			}
			var ep Endpoint
			if err := db.Where("provider_id = ?", p.ID).First(&ep).Error; err != nil {
				t.Fatalf("endpoint 未生成: %v", err)
			}
			if ep.Protocol != tt.wantProtocol {
				t.Fatalf("endpoint.Protocol = %q, want %q", ep.Protocol, tt.wantProtocol)
			}
		})
	}
}

// TestMigrateLegacyProviders_NoAPIKey 锁定 Config 无 api_key 时：生成分组但不建凭据，
// Config 不被改写。
func TestMigrateLegacyProviders_NoAPIKey(t *testing.T) {
	db := testChannelDB(t)
	legacy := &Provider{
		Name:   "no-key",
		Type:   "openai",
		Config: `{"base_url":"https://api.example.com/v1"}`,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	migrateLegacyProviders(context.Background(), db, testChannelCipher(t))

	var groups, creds int64
	db.Model(&KeyGroup{}).Where("provider_id = ?", legacy.ID).Count(&groups)
	db.Model(&Credential{}).Count(&creds)
	if groups != 1 {
		t.Fatalf("分组数 = %d, want 1", groups)
	}
	if creds != 0 {
		t.Fatalf("凭据数 = %d, want 0", creds)
	}
	var got Provider
	db.First(&got, legacy.ID)
	if got.Config != legacy.Config {
		t.Fatalf("无 api_key 的 Config 被改写: %s → %s", legacy.Config, got.Config)
	}
}

// TestMigrateLegacyProviders_NilCipherPanics 锁定「有 Provider 但未注入 cipher 必须 panic」，
// 禁止半迁移态（明文既没搬也没清空却静默继续）。
func TestMigrateLegacyProviders_NilCipherPanics(t *testing.T) {
	db := testChannelDB(t)
	if err := db.Create(&Provider{Name: "legacy", Type: "openai", Config: `{"api_key":"sk-x"}`}).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("nil cipher + 存量 Provider 未 panic")
		}
	}()
	migrateLegacyProviders(context.Background(), db, nil)
}

// TestMigrateLegacyProviders_EmptyDBNoCipherOK 锁定空库在无 cipher 时正常通过
// （testsupport 等测试场景不注入 cipher 也不会炸）。
func TestMigrateLegacyProviders_EmptyDBNoCipherOK(t *testing.T) {
	db := testChannelDB(t)
	migrateLegacyProviders(context.Background(), db, nil)
}
