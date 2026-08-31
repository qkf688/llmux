package channel

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// TestSelectCredential_Filter 锁 AC-4 凭据过滤：
// disabled / error / temp_unsched（鉴权判停，#6-2）/ 冷却中（CooldownUntil > now）
// 一律不可选，可用的只剩 active。
func TestSelectCredential_Filter(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Provider: models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {
				{KeyHash: "h-active"},
				{KeyHash: "h-disabled", Status: models.CredentialStatusDisabled},
				{KeyHash: "h-error", Status: models.CredentialStatusError},
				{KeyHash: "h-temputsched", Status: models.CredentialStatusTempUnsched},
				{KeyHash: "h-cooldown", CooldownUntil: ptrTime(now.Add(5 * time.Minute))},
				{KeyHash: "h-expired", CooldownUntil: ptrTime(now.Add(-5 * time.Minute))},
			},
		},
	}
	for _, tt := range []struct {
		name string
		now  time.Time
		want string
	}{
		{name: "正常时刻选 active", now: now, want: "h-active"},
		// 冷却全部过期后 expired 也进入候选，轮询按表序遍历：第一条 active 仍被选中（轮询从 0 开始）
		{name: "冷却过期后候选池含 expired", now: now.Add(10 * time.Minute), want: "h-active"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := SelectCredential(&Selector{}, snap, 1, tt.now)
			if err != nil {
				t.Fatalf("SelectCredential error: %v", err)
			}
			if c.KeyHash != tt.want {
				t.Fatalf("SelectCredential = %s, want %s", c.KeyHash, tt.want)
			}
		})
	}
}

// TestSelectCredential_AllUnusable 锁「全部不可用 → ErrNoCredentialAvailable」。
func TestSelectCredential_AllUnusable(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Provider: models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {
				{KeyHash: "h-cooldown", CooldownUntil: ptrTime(now.Add(time.Hour))},
				{KeyHash: "h-disabled", Status: models.CredentialStatusDisabled},
			},
		},
	}
	_, err := SelectCredential(&Selector{}, snap, 1, now)
	if !errors.Is(err, ErrNoCredentialAvailable) {
		t.Fatalf("err = %v, want %v", err, ErrNoCredentialAvailable)
	}
}

// TestSelectCredential_NoCandidates 锁「分组无凭据（地图无键）→ ErrNoCredentialAvailable」。
func TestSelectCredential_NoCandidates(t *testing.T) {
	snap := &Snapshot{
		Provider:           models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{},
	}
	_, err := SelectCredential(&Selector{}, snap, 99, time.Now())
	if !errors.Is(err, ErrNoCredentialAvailable) {
		t.Fatalf("err = %v, want %v", err, ErrNoCredentialAvailable)
	}
}

// TestSelectCredential_RoundRobin 锁 AC-4 组内轮询：3 条可用凭据连续选 3 次
// 各命中一次（选择即推进，不重复不跳过）。
func TestSelectCredential_RoundRobin(t *testing.T) {
	snap := &Snapshot{
		Provider: models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{KeyHash: "h1"}, {KeyHash: "h2"}, {KeyHash: "h3"}},
		},
	}
	s := &Selector{}
	seen := map[string]int{}
	for range 3 {
		c, err := SelectCredential(s, snap, 1, time.Now())
		if err != nil {
			t.Fatalf("SelectCredential error: %v", err)
		}
		seen[c.KeyHash]++
	}
	if len(seen) != 3 {
		t.Fatalf("3 次选择只命中 %d 条凭据: %v", len(seen), seen)
	}
	for hash, n := range seen {
		if n != 1 {
			t.Fatalf("凭据 %s 命中 %d 次, want 1", hash, n)
		}
	}
}

// TestSelectCredential_CooldownBoundary 锁冷却边界：CooldownUntil 恰等于 now 视为
// 已到期可用——实现用 After(now) 严格大于判定冷却中，到期即自动恢复、不占状态位，
// 边界值必须落在「可用」一侧。
func TestSelectCredential_CooldownBoundary(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Provider: models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{KeyHash: "h-boundary", CooldownUntil: &now}},
		},
	}
	c, err := SelectCredential(&Selector{}, snap, 1, now)
	if err != nil {
		t.Fatalf("SelectCredential error: %v", err)
	}
	if c.KeyHash != "h-boundary" {
		t.Fatalf("SelectCredential = %s, want h-boundary（CooldownUntil == now 应可用）", c.KeyHash)
	}
}

// TestSelectCredential_CoolingEnds 锁「冷却到期自动恢复」：CooldownUntil 过期后
// 凭据重新进入候选池（不占状态位，到期即恢复——设计定案第 5 节）。
func TestSelectCredential_CoolingEnds(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Provider: models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{KeyHash: "h-only", CooldownUntil: ptrTime(now.Add(-time.Minute))}},
		},
	}
	c, err := SelectCredential(&Selector{}, snap, 1, now)
	if err != nil {
		t.Fatalf("SelectCredential error: %v", err)
	}
	if c.KeyHash != "h-only" {
		t.Fatalf("SelectCredential = %s, want h-only", c.KeyHash)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// TestRetryCredential_SkipsCooldown_LocksGroupAndEndpoint：组内故障转移（#13）
// 的 channel 侧契约——重选时冷却中的凭据被剔除、组/端点锁定不换、Config 用新凭据
// 明文与端点 URL 重建。生产调用方在一次凭据级失败写库冷却后重新 Assemble 刷新
// 快照，本用例以「构造已冷却快照」等价模拟，聚焦选择语义本身。
func TestRetryCredential_SkipsCooldown_LocksGroupAndEndpoint(t *testing.T) {
	cipher := setupCipherForChannel(t)
	now := time.Now()

	groupID := uint(3)
	endpointID := uint(5)
	enc1, err := cipher.Encrypt("sk-key1")
	if err != nil {
		t.Fatalf("encrypt key1: %v", err)
	}
	enc2, err := cipher.Encrypt("sk-key2")
	if err != nil {
		t.Fatalf("encrypt key2: %v", err)
	}
	cooldownUntil := now.Add(10 * time.Minute)

	mkSnap := func(cooldown map[uint]*time.Time) *Snapshot {
		return &Snapshot{
			Provider: models.Provider{
				Model:  gorm.Model{ID: 1},
				Type:   "openai",
				Config: `{"base_url":"https://base.example/v1","version":"2023-06-01"}`,
			},
			Endpoints: []models.Endpoint{
				{Model: gorm.Model{ID: endpointID}, ProviderID: 1, Protocol: "openai", URL: "https://ep.example/claude/v1", Enabled: true},
			},
			Groups: []models.KeyGroup{
				{Model: gorm.Model{ID: groupID}, ProviderID: 1, Name: "g1", Weight: 1},
			},
			CredentialsByGroup: map[uint][]models.Credential{
				groupID: {
					// 按 ID ASC 候选顺序：key1 在前，轮询首现命中 key1
					{Model: gorm.Model{ID: 11}, GroupID: &groupID, Key: enc1, KeyHash: cipher.Hash("sk-key1"), CooldownUntil: cooldown[11]},
					{Model: gorm.Model{ID: 12}, GroupID: &groupID, Key: enc2, KeyHash: cipher.Hash("sk-key2"), CooldownUntil: cooldown[12]},
				},
			},
		}
	}

	// ① 全可用 → 命中 key1（RR 首现）
	sel := &Selector{}
	res1, err := sel.RetryCredential(mkSnap(nil), groupID, models.Endpoint{Model: gorm.Model{ID: endpointID}, URL: "https://ep.example/claude/v1"}, now)
	if err != nil {
		t.Fatalf("retry step 1 error: %v", err)
	}
	if res1.Credential.KeyHash != cipher.Hash("sk-key1") {
		t.Fatalf("step 1 Credential = %s, want key1（轮询首现）", res1.Credential.KeyHash)
	}
	if res1.Endpoint.ID != endpointID || res1.Group.ID != groupID {
		t.Fatalf("step 1 组/端点被换：endpoint=%d group=%d, want %d/%d（锁组重选不换）", res1.Endpoint.ID, res1.Group.ID, endpointID, groupID)
	}
	if !strings.Contains(res1.Config, `"api_key":"sk-key1"`) {
		t.Fatalf("step 1 config 缺 key1 明文: %s", res1.Config)
	}
	if res1.UpstreamURL != "https://ep.example/claude/v1" {
		t.Fatalf("step 1 UpstreamURL = %q, want 端点覆盖 URL（继承链）", res1.UpstreamURL)
	}

	// ② key1 冷却 → 重选命中 key2（冷却剔除，同组内故障转移）
	sel = &Selector{}
	res2, err := sel.RetryCredential(mkSnap(map[uint]*time.Time{11: &cooldownUntil}), groupID, res1.Endpoint, now)
	if err != nil {
		t.Fatalf("retry step 2 error: %v", err)
	}
	if res2.Credential.KeyHash != cipher.Hash("sk-key2") {
		t.Fatalf("step 2 Credential = %s, want key2（key1 冷却中被剔除）", res2.Credential.KeyHash)
	}
	if !strings.Contains(res2.Config, `"api_key":"sk-key2"`) {
		t.Fatalf("step 2 config 缺 key2 明文: %s", res2.Config)
	}
	// ③ 全部冷却（组耗尽）→ ErrNoCredentialAvailable 上抛，由 chat 链路做组织级淘汰
	sel = &Selector{}
	if _, err := sel.RetryCredential(mkSnap(map[uint]*time.Time{11: &cooldownUntil, 12: &cooldownUntil}), groupID, res1.Endpoint, now); err == nil {
		t.Fatal("step 3 want error, got nil（组内无可用凭据必须上抛）")
	} else if !errors.Is(err, ErrNoCredentialAvailable) {
		t.Fatalf("step 3 err = %v, want ErrNoCredentialAvailable sentinel", err)
	}
}
