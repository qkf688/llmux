package channel

import (
	"errors"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

// TestSelectCredential_Filter 锁 AC-4 凭据过滤：
// disabled / error / 冷却中（CooldownUntil > now）一律不可选，可用的只剩 active。
func TestSelectCredential_Filter(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Provider: models.Provider{Type: "openai"},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {
				{KeyHash: "h-active"},
				{KeyHash: "h-disabled", Status: models.CredentialStatusDisabled},
				{KeyHash: "h-error", Status: models.CredentialStatusError},
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
