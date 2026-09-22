package channel

import (
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// newPreferSnapshot 构造两分组×两凭据的选路快照（分组权重相同、凭据按 ID ASC），
// 供 Prefer 偏好语义测试：钉定命中、失效回退、不推进轮询指针均可在此快照上确定性观察。
func newPreferSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	cipher := setupCipherForChannel(t)
	group1, group2 := uint(1), uint(2)

	enc := func(plain string) string {
		e, err := cipher.Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt %s: %v", plain, err)
		}
		return e
	}
	cred := func(id uint, groupID uint, plain string) models.Credential {
		gid := groupID
		return models.Credential{
			Model:   gorm.Model{ID: id},
			GroupID: &gid,
			Key:     enc(plain),
			KeyHash: cipher.Hash(plain),
		}
	}

	return &Snapshot{
		Provider: models.Provider{
			Model:  gorm.Model{ID: 1},
			Type:   "openai",
			Config: `{"base_url":"https://base.example/v1","version":"2023-06-01"}`,
		},
		Endpoints: []models.Endpoint{
			{Model: gorm.Model{ID: 5}, ProviderID: 1, Protocol: string(consts.ProtocolOpenAI), URL: "https://ep.example/v1", Enabled: true},
		},
		Groups: []models.KeyGroup{
			{Model: gorm.Model{ID: group1}, ProviderID: 1, Name: "g1", Weight: 1},
			{Model: gorm.Model{ID: group2}, ProviderID: 1, Name: "g2", Weight: 1},
		},
		CredentialsByGroup: map[uint][]models.Credential{
			group1: {cred(11, group1, "sk-11"), cred(12, group1, "sk-12")},
			group2: {cred(21, group2, "sk-21"), cred(22, group2, "sk-22")},
		},
	}
}

// TestSelect_PreferPinsGroupAndCredential 锁 AC-1 钉定命中：Prefer 命中时无视
// 权重随机/轮询的起始位置，恒选钉定的分组与凭据（重复请求稳定回同一账号 = 缓存亲和）。
func TestSelect_PreferPinsGroupAndCredential(t *testing.T) {
	snap := newPreferSnapshot(t)
	now := time.Now()
	s := &Selector{}
	prefer := Prefer{GroupID: 2, CredentialID: 22}

	for i := range 2 {
		res, err := s.Select(snap, consts.FormatOpenAIChat, "m", now, prefer)
		if err != nil {
			t.Fatalf("Select #%d error: %v", i+1, err)
		}
		if res.Group.ID != 2 || res.Credential.ID != 22 {
			t.Fatalf("Select #%d = group %d credential %d, want 2/22（钉定命中）", i+1, res.Group.ID, res.Credential.ID)
		}
	}
}

// TestSelect_PreferInvalidFallsBack 锁 AC-2 静默回退：钉定目标失效时按现状选择，
// 且各层独立回退——凭据失效不连累分组钉定，分组失效才整体回退。
func TestSelect_PreferInvalidFallsBack(t *testing.T) {
	now := time.Now()

	t.Run("凭据冷却中：分组仍钉住，凭据回落轮询", func(t *testing.T) {
		snap := newPreferSnapshot(t)
		cooling := now.Add(time.Hour)
		snap.CredentialsByGroup[2][1].CooldownUntil = &cooling // 钉定的 22 冷却

		res, err := (&Selector{}).Select(snap, consts.FormatOpenAIChat, "m", now, Prefer{GroupID: 2, CredentialID: 22})
		if err != nil {
			t.Fatalf("Select error: %v", err)
		}
		if res.Group.ID != 2 || res.Credential.ID != 21 {
			t.Fatalf("= group %d credential %d, want 2/21（组钉住、凭据回退轮询首现）", res.Group.ID, res.Credential.ID)
		}
	})

	t.Run("分组凭据全冷却：整体回落常规选择", func(t *testing.T) {
		snap := newPreferSnapshot(t)
		cooling := now.Add(time.Hour)
		for i := range snap.CredentialsByGroup[2] {
			snap.CredentialsByGroup[2][i].CooldownUntil = &cooling
		}

		res, err := (&Selector{}).Select(snap, consts.FormatOpenAIChat, "m", now, Prefer{GroupID: 2, CredentialID: 22})
		if err != nil {
			t.Fatalf("Select error: %v", err)
		}
		if res.Group.ID != 1 || res.Credential.ID != 11 {
			t.Fatalf("= group %d credential %d, want 1/11（回落常规选择，全新 Selector 轮询首现）", res.Group.ID, res.Credential.ID)
		}
	})

	t.Run("分组被白名单排除：整体回落常规选择", func(t *testing.T) {
		snap := newPreferSnapshot(t)
		snap.Groups[1].Models = "other-model"

		res, err := (&Selector{}).Select(snap, consts.FormatOpenAIChat, "m", now, Prefer{GroupID: 2, CredentialID: 22})
		if err != nil {
			t.Fatalf("Select error: %v", err)
		}
		if res.Group.ID != 1 {
			t.Fatalf("= group %d, want 1（白名单外的钉定组不得被钉住）", res.Group.ID)
		}
	})

	t.Run("钉定 ID 不存在：整体回落常规选择", func(t *testing.T) {
		snap := newPreferSnapshot(t)
		res, err := (&Selector{}).Select(snap, consts.FormatOpenAIChat, "m", now, Prefer{GroupID: 99, CredentialID: 99})
		if err != nil {
			t.Fatalf("Select error: %v", err)
		}
		if res.Group.ID != 1 || res.Credential.ID != 11 {
			t.Fatalf("= group %d credential %d, want 1/11（全新 Selector 轮询首现）", res.Group.ID, res.Credential.ID)
		}
	})
}

// TestSelect_PreferDoesNotAdvanceRR 锁 AC-3「钉定命中不推进轮询指针」：钉定流量
// 不得挪动常规流量的分摊位置——一次钉定命中后，零值 Select 仍从轮询首现出发。
func TestSelect_PreferDoesNotAdvanceRR(t *testing.T) {
	snap := newPreferSnapshot(t)
	now := time.Now()
	s := &Selector{}

	if _, err := s.Select(snap, consts.FormatOpenAIChat, "m", now, Prefer{GroupID: 2, CredentialID: 22}); err != nil {
		t.Fatalf("pinned Select error: %v", err)
	}

	// 分组轮询指针未动：零值 Prefer 仍命中首现 g1（若钉定推进过指针则会跳到 g2）。
	res, err := s.Select(snap, consts.FormatOpenAIChat, "m", now, Prefer{})
	if err != nil {
		t.Fatalf("plain Select error: %v", err)
	}
	if res.Group.ID != 1 {
		t.Fatalf("plain Select group = %d, want 1（钉定命中不得推进分组轮询）", res.Group.ID)
	}

	// 凭据轮询指针未动：g2 组内仍首现 21。
	c, err := SelectCredential(s, snap, 2, now)
	if err != nil {
		t.Fatalf("SelectCredential error: %v", err)
	}
	if c.ID != 21 {
		t.Fatalf("SelectCredential = %d, want 21（钉定命中不得推进凭据轮询）", c.ID)
	}
}
