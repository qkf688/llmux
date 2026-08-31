package channel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// reposFromDB 直接用内存库构造 Repositories（autoassoc 模式：不经包级默认）。
func reposFromDB(db *gorm.DB) *repository.Repositories { return repository.New(db) }

// newAssembleTestDB 内存库 + 仅装配所需三表（credentials/endpoints/key_groups）。
// pools 表不需要：凭据筛选无 Join，PoolID 只是 index 列（无外键约束）。
func newAssembleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:assemble_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if err := db.AutoMigrate(&models.Credential{}, &models.Endpoint{}, &models.KeyGroup{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

// TestAssemble_CredentialMerge 锁装配归并语义：
//
//	内联凭据（GroupID）与号池凭据（PoolID 经分组引用）两侧都归入该分组；
//	组内顺序按 ID ASC 稳定（轮询取模的前提）；无归属凭据的分组为空切片且键存在。
func TestAssemble_CredentialMerge(t *testing.T) {
	db := newAssembleTestDB(t)
	ctx := context.Background()

	if err := db.Create(&[]models.Endpoint{
		{ProviderID: 1, Protocol: "openai", Enabled: true},
		{ProviderID: 1, Protocol: "anthropic", Enabled: true},
	}).Error; err != nil {
		t.Fatalf("create endpoints: %v", err)
	}
	if err := db.Create(&[]models.KeyGroup{
		{ProviderID: 1, Name: "内联组", Weight: 1},
		{ProviderID: 1, Name: "号池组", Weight: 2, PoolID: uintPtr(9)},
		{ProviderID: 1, Name: "空组", Weight: 1},
	}).Error; err != nil {
		t.Fatalf("create groups: %v", err)
	}
	// 创建（单条依次）：c3 先插得 ID=1、c1 后插得 ID=2——期望装配输出按
	// ID ASC = [h3, h1]（组内候选顺序与插入无关，稳定升序是轮询取模的前提）
	if err := db.Create(&models.Credential{Key: "c3", KeyHash: "h3", GroupID: uintPtr(1)}).Error; err != nil {
		t.Fatalf("create c3: %v", err)
	}
	if err := db.Create(&models.Credential{Key: "c1", KeyHash: "h1", GroupID: uintPtr(1)}).Error; err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := db.Create(&models.Credential{Key: "c-pool", KeyHash: "h-pool", PoolID: uintPtr(9)}).Error; err != nil {
		t.Fatalf("create c-pool: %v", err)
	}

	repos := reposFromDB(db)
	snap, err := NewAssembler(repos).Assemble(ctx, models.Provider{Model: gorm.Model{ID: 1}, Type: "openai"})
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}

	if len(snap.Endpoints) != 2 {
		t.Fatalf("endpoints = %d, want 2", len(snap.Endpoints))
	}
	if len(snap.Groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(snap.Groups))
	}

	// 组 1：内联两条，按 ID ASC（c3 ID=1 在 c1 ID=2 之前）
	g1 := snap.CredentialsByGroup[1]
	if len(g1) != 2 {
		t.Fatalf("group 1 credentials = %d, want 2 (内联归并)", len(g1))
	}
	if g1[0].KeyHash != "h3" || g1[1].KeyHash != "h1" {
		t.Fatalf("group 1 order = [%s %s], want [h3 h1]（ID ASC 稳定）", g1[0].KeyHash, g1[1].KeyHash)
	}

	// 组 2：号池凭据归入（经 PoolID 引用）
	g2 := snap.CredentialsByGroup[2]
	if len(g2) != 1 || g2[0].KeyHash != "h-pool" {
		t.Fatalf("group 2 credentials = %v, want [h-pool]（号池归并）", g2)
	}

	// 空组：键存在、空切片（轮询判定「无可用」路径不 panic）
	if g, ok := snap.CredentialsByGroup[3]; !ok || len(g) != 0 {
		t.Fatalf("group 3 = (%v, %v), want (empty slice, true)", g, ok)
	}
}

// TestSelect_FullChain 锁 Select 门面端到端：三层命中 + 动态 config 组装
// （AC-1..AC-5 的组合路径）。透传 wire 选中对应端点，Config 含明文 key 与
// 继承链解析出的 URL。
func TestSelect_FullChain(t *testing.T) {
	setupCipherForChannel(t)

	snap := &Snapshot{
		Provider: models.Provider{
			Model:  gorm.Model{ID: 1},
			Type:   "openai",
			Config: `{"base_url":"https://default.example/v1"}`,
		},
		Endpoints: []models.Endpoint{
			{ProviderID: 1, Protocol: "openai", URL: "", Enabled: true},
			{ProviderID: 1, Protocol: "anthropic", URL: "https://anthropic.example/v1", Enabled: true},
		},
		Groups: []models.KeyGroup{
			{Model: gorm.Model{ID: 1}, ProviderID: 1, Name: "低价组", Weight: 5, Models: ""},
		},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{Key: mustEncrypt(t, "sk-secret-789")}},
		},
	}

	c := &Selector{}
	res, err := c.Select(snap, "openai", "gpt-4o", now())
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if res.Endpoint.Protocol != "openai" {
		t.Fatalf("endpoint = %q, want openai（透传路径）", res.Endpoint.Protocol)
	}
	if res.Group.Name != "低价组" {
		t.Fatalf("group = %q, want 低价组", res.Group.Name)
	}
	if res.UpstreamURL != "https://default.example/v1" {
		t.Fatalf("UpstreamURL = %q, want 继承 base_url", res.UpstreamURL)
	}
	if !containsStr(res.Config, `"api_key":"sk-secret-789"`) {
		t.Fatalf("config %s 缺少明文 api_key", res.Config)
	}
	if !containsStr(res.Config, `"base_url":"https://default.example/v1"`) {
		t.Fatalf("config %s 缺少 base_url", res.Config)
	}
}

// TestSelect_FallbackChain 锁转换路径端到端：入站 wire 无匹配 → 主协议端点，
// 端点 URL 覆盖 base_url。
func TestSelect_FallbackChain(t *testing.T) {
	setupCipherForChannel(t)

	snap := &Snapshot{
		Provider: models.Provider{
			Model:  gorm.Model{ID: 1},
			Type:   "openai",
			Config: `{"base_url":"https://default.example/v1"}`,
		},
		Endpoints: []models.Endpoint{
			{ProviderID: 1, Protocol: "anthropic", URL: "https://anthropic.example/claude/v1", Enabled: true},
			{ProviderID: 1, Protocol: "openai", URL: "", Enabled: true},
		},
		Groups: []models.KeyGroup{
			{Model: gorm.Model{ID: 1}, ProviderID: 1, Name: "默认组", Weight: 1, Models: ""},
		},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{Key: mustEncrypt(t, "sk-secret-789")}},
		},
	}

	res, err := (&Selector{}).Select(snap, "openai-res", "gpt-4o", now())
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if res.Endpoint.Protocol != "openai" {
		t.Fatalf("endpoint = %q, want 主协议 openai 端点（转换路径）", res.Endpoint.Protocol)
	}
	if res.UpstreamURL != "https://default.example/v1" {
		t.Fatalf("UpstreamURL = %q, want 主端点继承 base_url", res.UpstreamURL)
	}
}

// TestSelect_AllCooling 锁「全凭据冷却 → ErrNoCredentialAvailable 冒泡」
// （#13 据 sentinel 判定组内转移 vs 组织级失败）；同时锁 #6-3：凭据失败仍带回
// Endpoint/Group，供探活锁定刚耗尽的组（禁止调用方再 SelectGroup）。
func TestSelect_AllCooling(t *testing.T) {
	snap := &Snapshot{
		Provider:  models.Provider{Model: gorm.Model{ID: 1}, Type: "openai", Config: `{"base_url":"u"}`},
		Endpoints: []models.Endpoint{{Model: gorm.Model{ID: 9}, ProviderID: 1, Protocol: "openai", Enabled: true}},
		Groups:    []models.KeyGroup{{Model: gorm.Model{ID: 7}, ProviderID: 1, Name: "g", Weight: 1}},
		CredentialsByGroup: map[uint][]models.Credential{
			7: {{KeyHash: "h", CooldownUntil: futureTime()}},
		},
	}
	got, err := (&Selector{}).Select(snap, "openai", "m", now())
	if !errors.Is(err, ErrNoCredentialAvailable) {
		t.Fatalf("err = %v, want %v", err, ErrNoCredentialAvailable)
	}
	if got.Endpoint.ID != 9 || got.Group.ID != 7 {
		t.Fatalf("partial selection = ep=%d group=%d, want ep=9 group=7", got.Endpoint.ID, got.Group.ID)
	}
}

func uintPtr(v uint) *uint           { return &v }
func now() time.Time                 { return time.Now() }
func futureTime() *time.Time         { t := time.Now().Add(time.Hour); return &t }
func containsStr(s, sub string) bool { return strings.Contains(s, sub) }
func mustEncrypt(t *testing.T, plain string) string {
	t.Helper()
	enc, err := credentialcrypto.Default().Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	return enc
}
