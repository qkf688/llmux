package channel

import (
	"errors"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// newTestSnapshot 构造一个可复用的选路快照：openai + anthropic 两个 enabled 端点、
// 两个分组（默认组 w1 + 便宜组 w2，白名单分别为空与命中模型）、各自一条凭据。
func newTestSnapshot() *Snapshot {
	return &Snapshot{
		Provider: models.Provider{
			Model:  gorm.Model{ID: 1},
			Type:   "openai",
			Config: `{"base_url":"https://default.example/v1"}`,
		},
		Endpoints: []models.Endpoint{
			{ProviderID: 1, Protocol: string(consts.ProtocolOpenAI), URL: "", Enabled: true},
			{ProviderID: 1, Protocol: string(consts.ProtocolAnthropic), URL: "", Enabled: true},
		},
		Groups: []models.KeyGroup{
			{ProviderID: 1, Name: "默认组", Weight: 1, Models: ""},
			{ProviderID: 1, Name: "便宜组", Weight: 2, Models: "cheap-model"},
		},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{KeyHash: "h-default"}},
			2: {{KeyHash: "h-cheap"}},
		},
	}
}

// TestSelectEndpoint_Passthrough 锁 AC-1 透传路径：入站 wire 与 enabled 端点协议
// 匹配时选中该端点（openai 客户端 → openai 端点），不理会端点顺序。
func TestSelectEndpoint_Passthrough(t *testing.T) {
	snap := newTestSnapshot()
	for _, tt := range []struct {
		name   string
		wire   consts.WireFormat
		wantEp string
	}{
		{name: "openai 入站", wire: "openai", wantEp: string(consts.ProtocolOpenAI)},
		{name: "anthropic 入站", wire: "anthropic", wantEp: string(consts.ProtocolAnthropic)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ep, err := SelectEndpoint(snap, tt.wire)
			if err != nil {
				t.Fatalf("SelectEndpoint(%q) error: %v", tt.wire, err)
			}
			if ep.Protocol != tt.wantEp {
				t.Fatalf("SelectEndpoint(%q) = endpoint %q, want %q", tt.wire, ep.Protocol, tt.wantEp)
			}
		})
	}
}

// TestSelectEndpoint_FallbackToPrimary 锁 AC-1 转换路径：入站 wire 无匹配时
// 选中主协议端点（Provider.Type 派生协议），即使它排在其它端点后面。
func TestSelectEndpoint_FallbackToPrimary(t *testing.T) {
	snap := newTestSnapshot()
	// responses 入站 wire：openai/anthropic 端点都不匹配 → 回落到主协议（openai）端点
	ep, err := SelectEndpoint(snap, "openai-res")
	if err != nil {
		t.Fatalf("SelectEndpoint(responses) error: %v", err)
	}
	if ep.Protocol != string(consts.ProtocolOpenAI) {
		t.Fatalf("SelectEndpoint(responses) = %q, want primary openai endpoint", ep.Protocol)
	}
}

// TestSelectEndpoint_DisabledExcluded 锁「disabled 端点不参与任何选择」：
// 唯一 enabled 端点被禁后，透传与转换路径都应报 ErrEndpointUnavailable。
func TestSelectEndpoint_DisabledExcluded(t *testing.T) {
	snap := newTestSnapshot()
	for i := range snap.Endpoints {
		snap.Endpoints[i].Enabled = false
	}

	// 透传路径：入站 openai 无 enabled 匹配
	_, err := SelectEndpoint(snap, "openai")
	if !errors.Is(err, ErrEndpointUnavailable) {
		t.Fatalf("passthrough err = %v, want %v", err, ErrEndpointUnavailable)
	}
	// 转换路径：主协议端点（openai）已停用
	_, err = SelectEndpoint(snap, "openai-res")
	if !errors.Is(err, ErrEndpointUnavailable) {
		t.Fatalf("fallback err = %v, want %v", err, ErrEndpointUnavailable)
	}
}

// TestSelectEndpoint_UnknownProtocolSkipped 锁「未知协议端点被跳过」：端点协议
// 不在 consts.Protocol* 集合时既不匹配透传、也不当主协议端点（宁可报错不静默适配）。
func TestSelectEndpoint_UnknownProtocolSkipped(t *testing.T) {
	snap := newTestSnapshot()
	snap.Endpoints = []models.Endpoint{
		{ProviderID: 1, Protocol: "claude-v1", URL: "", Enabled: true},
	}
	_, err := SelectEndpoint(snap, "openai")
	if !errors.Is(err, ErrEndpointUnavailable) {
		t.Fatalf("err = %v, want %v", err, ErrEndpointUnavailable)
	}
}

// TestSelectGroup_Whitelist 锁 AC-3 白名单过滤（空 = 不限）：
//   - 白名单空的分组任何模型都可选
//   - 白名单命中的分组可选
//   - 白名单非空且不包含请求模型的分组被排除（即使权重更高）
func TestSelectGroup_Whitelist(t *testing.T) {
	s := &Selector{}

	// 空白名单组 + 命中组都在 → 选任意一个都合法，断言不报错
	snap := newTestSnapshot()
	g, err := SelectGroup(s, snap, "cheap-model")
	if err != nil {
		t.Fatalf("SelectGroup(cheap-model) error: %v", err)
	}
	if g.Name == "" {
		t.Fatal("SelectGroup returned empty group")
	}

	// 白名单不命中 → 只剩"默认组"可选
	g, err = SelectGroup(s, newTestSnapshot(), "unknown-model")
	if err != nil {
		t.Fatalf("SelectGroup(unknown-model) error: %v", err)
	}
	if g.Name != "默认组" {
		t.Fatalf("SelectGroup(unknown-model) = %q, want 默认组", g.Name)
	}
}

// TestSelectGroup_AllFiltered 锁「白名单全滤掉 → ErrNoGroupMatches」。
func TestSelectGroup_AllFiltered(t *testing.T) {
	snap := newTestSnapshot()
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "定向组", Weight: 1, Models: "only-this-model"},
	}
	_, err := SelectGroup(&Selector{}, snap, "other-model")
	if !errors.Is(err, ErrNoGroupMatches) {
		t.Fatalf("err = %v, want %v", err, ErrNoGroupMatches)
	}
}

// TestSelectGroup_ZeroWeightExcluded 锁「weight <= 0 的分组不参与（前端空输入落 0）」。
func TestSelectGroup_ZeroWeightExcluded(t *testing.T) {
	snap := newTestSnapshot()
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "未配权重", Weight: 0, Models: ""},
	}
	_, err := SelectGroup(&Selector{}, snap, "any-model")
	if !errors.Is(err, ErrNoGroupMatches) {
		t.Fatalf("err = %v, want %v", err, ErrNoGroupMatches)
	}
}

// TestSelectGroup_WhitelistSpacing 锁白名单空格语义：条目前后空格被裁剪后参与
// 精确匹配；纯空格白名单按「空 = 不限」处理（空格不因 TrimSpace 留痕而成为条目）。
// 白名单空格处理是任务点名的边界（空串/空格不误判为「不限」也不误把空格当条目）。
func TestSelectGroup_WhitelistSpacing(t *testing.T) {
	// 带空格条目：裁剪后命中
	snap := newTestSnapshot()
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "带空格组", Weight: 1, Models: " cheap-model , gpt-4o "},
	}
	g, err := SelectGroup(&Selector{}, snap, "cheap-model")
	if err != nil {
		t.Fatalf("SelectGroup(cheap-model) error: %v", err)
	}
	if g.Name != "带空格组" {
		t.Fatalf("SelectGroup = %q, want 带空格组（条目空格应裁剪）", g.Name)
	}

	// 纯空格白名单 = 不限：任意模型可入，不得把空白串当成一个不匹配条目
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "纯空格组", Weight: 1, Models: "   "},
	}
	if _, err := SelectGroup(&Selector{}, snap, "anything"); err != nil {
		t.Fatalf("纯空格白名单应视为不限: %v", err)
	}
}

// TestSelectGroup_SameWeightRoundRobin 锁 AC-3 同权重分组间轮询：
// 三个 weight=1 分组连续选 3 次各命中一次（轮询不重复不跳过）。
func TestSelectGroup_SameWeightRoundRobin(t *testing.T) {
	snap := newTestSnapshot()
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "A", Weight: 1, Models: ""},
		{ProviderID: 1, Name: "B", Weight: 1, Models: ""},
		{ProviderID: 1, Name: "C", Weight: 1, Models: ""},
	}
	s := &Selector{}
	seen := map[string]int{}
	for range 3 {
		g, err := SelectGroup(s, snap, "m")
		if err != nil {
			t.Fatalf("SelectGroup error: %v", err)
		}
		seen[g.Name]++
	}
	if len(seen) != 3 {
		t.Fatalf("3 次选择只命中 %d 个不同分组: %v", len(seen), seen)
	}
	for name, n := range seen {
		if n != 1 {
			t.Fatalf("分组 %q 命中 %d 次, want 1", name, n)
		}
	}
}

// TestSelectGroup_WeightDistribution 锁「档间加权随机按单组权重分配」：
// weight=3 的组应显著比 weight=1 的组更常被选（100 次中 > 60 次）。
func TestSelectGroup_WeightDistribution(t *testing.T) {
	snap := newTestSnapshot()
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "低权", Weight: 1, Models: ""},
		{ProviderID: 1, Name: "高权", Weight: 3, Models: ""},
	}
	s := &Selector{}
	hits := map[string]int{}
	for range 100 {
		g, err := SelectGroup(s, snap, "m")
		if err != nil {
			t.Fatalf("SelectGroup error: %v", err)
		}
		hits[g.Name]++
	}
	if hits["高权"] <= 60 {
		t.Fatalf("高权组命中 %d/100, want > 60（加权随机显著偏差）", hits["高权"])
	}
	if hits["低权"] == 0 {
		t.Fatalf("低权组 100 次全未命中, want 至少命中（轮询兜底低概率组）")
	}
}
