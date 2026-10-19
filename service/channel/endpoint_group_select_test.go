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

// groupMatchesModel 的空条目口径：白名单里的空条目（"cheap-model,,"）不是成员——与走
// 同一 CSV 格式的 modelsync.splitModelsCSV（丢弃空条目）一致。放行空条目会让「关联漏填
// 上游名」与「手改白名单留了尾逗号」两处错误相抵：判定通过、一路走到上游才炸，报错点
// 远离真因。
func TestSelectGroup_EmptyEntryNotAMember(t *testing.T) {
	snap := newTestSnapshot()
	snap.Groups = []models.KeyGroup{
		{ProviderID: 1, Name: "含空条目", Weight: 1, Models: "cheap-model,,"},
	}

	// 上游名命中非空条目 → 可选
	if g, err := SelectGroup(&Selector{}, snap, "cheap-model"); err != nil || g.Name != "含空条目" {
		t.Fatalf("SelectGroup(cheap-model) = %q, err = %v, want 含空条目", g.Name, err)
	}
	// 上游名为空 → 不得与空条目相等而放行
	if _, err := SelectGroup(&Selector{}, snap, ""); !errors.Is(err, ErrNoGroupMatches) {
		t.Fatalf("err = %v, want %v（空条目不算成员，空上游名不可选）", err, ErrNoGroupMatches)
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

// TestSelectPrimaryEndpoint_PrefersMatchingEnabledMinID 锁诊断链路（同步/浏览/探活）的
// 端点口径：启用 + 协议匹配 Provider.Type 派生的主协议 + ID 最小。行为与原
// modelsync.selectSyncEndpoint 一致（收编为共享函数，不改变口径）。
func TestSelectPrimaryEndpoint_PrefersMatchingEnabledMinID(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolAnthropic), Enabled: true},
		{Model: gorm.Model{ID: 2}, Protocol: string(consts.ProtocolOpenAI), Enabled: false},
		{Model: gorm.Model{ID: 3}, Protocol: string(consts.ProtocolOpenAI), Enabled: true},
		{Model: gorm.Model{ID: 4}, Protocol: string(consts.ProtocolOpenAI), Enabled: true},
	}

	got, ok := SelectPrimaryEndpoint("openai", eps, nil)
	if !ok {
		t.Fatal("want enabled openai endpoint")
	}
	if got.ID != 3 {
		t.Fatalf("ID = %d, want 3（跳过停用端点，取 ID 最小可用者）", got.ID)
	}

	if got, ok := SelectPrimaryEndpoint("anthropic", eps, nil); !ok || got.ID != 1 {
		t.Fatalf("anthropic 主协议 = %+v (ok=%v), want ID 1", got, ok)
	}
	// responses 协议无端点：调用方（同步 SkipReason / 浏览报错）据此自行措辞。
	if _, ok := SelectPrimaryEndpoint("openai-res", eps, nil); ok {
		t.Fatal("无 responses 端点时必须返回 false")
	}
	// 未知 type 走 ProtocolOfType 的 openai 兜底（与 SelectEndpoint 同一张表，不另立规则）。
	if got, ok := SelectPrimaryEndpoint("mystery-type", eps, nil); !ok || got.ID != 3 {
		t.Fatalf("未知 type = %+v (ok=%v), want 兜底 openai ID 3", got, ok)
	}
	if _, ok := SelectPrimaryEndpoint("openai", eps[:2], nil); ok {
		t.Fatal("同协议端点全停用时必须返回 false")
	}
}

// TestGroupCandidates_IgnoresWhitelistAndWeight 锁探活/同步的遍历口径：**全部**分组、
// ID ASC，且白名单与 weight 都不参与过滤——白名单正是靠这次同步才填上的（按它过滤＝
// 先有鸡还是先有蛋），weight<=0 的组凭据仍是真 key，漏测就永远看不到它授权的上游模型。
func TestGroupCandidates_IgnoresWhitelistAndWeight(t *testing.T) {
	snap := &Snapshot{Groups: []models.KeyGroup{
		{Model: gorm.Model{ID: 5}, Name: "g5", Weight: 0, Models: "other"},
		{Model: gorm.Model{ID: 2}, Name: "g2", Weight: 3, Models: ""},
		{Model: gorm.Model{ID: 9}, Name: "g9", Weight: -1, Models: "m"},
	}}

	got := GroupCandidates(snap)
	if len(got) != 3 {
		t.Fatalf("候选数 = %d, want 3（weight/白名单不得过滤）", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 5 || got[2].ID != 9 {
		t.Fatalf("顺序 = %v, want ID ASC [2 5 9]", []uint{got[0].ID, got[1].ID, got[2].ID})
	}

	// 返回副本：调用方就地改名/重排不得污染快照（快照同时供生产选路读取）。
	got[0].Name = "mutated"
	if snap.Groups[1].Name == "mutated" {
		t.Fatal("返回了快照底层切片，遍历方的修改污染了快照")
	}

	if len(GroupCandidates(&Snapshot{})) != 0 {
		t.Fatal("空快照必须返回空候选，而不是 nil 语义混淆")
	}
}

// TestGroupCandidatesForModel_WhitelistOnlyDeterministic 锁管理端连通性测试的分组口径：
// 只按白名单过滤（空 = 不限）、ID ASC、weight 不参与，且限本供应商——与
// GroupCandidates（不过滤白名单，供同步/浏览逐组拉目录）刻意分道。
func TestGroupCandidatesForModel_WhitelistOnlyDeterministic(t *testing.T) {
	snap := &Snapshot{
		Provider: models.Provider{Model: gorm.Model{ID: 1}},
		Groups: []models.KeyGroup{
			{Model: gorm.Model{ID: 7}, ProviderID: 1, Name: "白名单含它", Weight: 0, Models: "other, m"},
			{Model: gorm.Model{ID: 3}, ProviderID: 1, Name: "空白名单=不限", Weight: 5, Models: ""},
			{Model: gorm.Model{ID: 4}, ProviderID: 1, Name: "白名单不含", Weight: 9, Models: "m2"},
			{Model: gorm.Model{ID: 2}, ProviderID: 2, Name: "别家供应商组", Weight: 9, Models: ""},
		},
	}

	got := GroupCandidatesForModel(snap, "m")
	if len(got) != 2 {
		t.Fatalf("候选数 = %d, want 2（白名单命中的两组；weight<=0 不排可用性）", len(got))
	}
	if got[0].ID != 3 || got[1].ID != 7 {
		t.Fatalf("顺序 = %v, want ID ASC [3 7]", []uint{got[0].ID, got[1].ID})
	}
	// 快照隔离：遍历方就地改名不得污染快照（同一份快照还要给生产选路读）。
	got[0].Name = "mutated"
	if snap.Groups[1].Name == "mutated" {
		t.Fatal("返回了快照底层切片的引用，修改污染了快照")
	}

	m2 := GroupCandidatesForModel(snap, "m2")
	if len(m2) != 2 {
		t.Fatalf("候选数 = %d, want 2（空白名单=不限的组 + 显式含 m2 的组）", len(m2))
	}
	only := GroupCandidatesForModel(snap, "m3")
	if len(only) != 1 || only[0].ID != 3 {
		t.Fatalf("候选 = %d 组, want 仅 ID 3 一组（白名单显式不含 m3 的组必须被过滤掉）", len(only))
	}
	if len(GroupCandidatesForModel(&Snapshot{}, "m")) != 0 {
		t.Fatal("空快照必须返回空候选")
	}
}

// customModelSnapshot 供应商声明了自定义模型 cm-1（Provider.Config.custom_models），
// 但两个分组的白名单都非空且**不含**它——正是「上游 /models 不返回、只在自定义清单里」
// 的场景（2026-09-28 拍板：自定义模型 = 供应商级声明授权，全部分组可服务）。
func customModelSnapshot() *Snapshot {
	return &Snapshot{
		Provider: models.Provider{
			Model:  gorm.Model{ID: 1},
			Type:   "openai",
			Config: `{"base_url":"https://default.example/v1","custom_models":["cm-1"]}`,
		},
		Endpoints: []models.Endpoint{
			{ProviderID: 1, Protocol: string(consts.ProtocolOpenAI), URL: "", Enabled: true},
		},
		Groups: []models.KeyGroup{
			{Model: gorm.Model{ID: 1}, ProviderID: 1, Name: "组A", Weight: 1, Models: "other-1"},
			{Model: gorm.Model{ID: 2}, ProviderID: 1, Name: "组B", Weight: 2, Models: "other-2"},
		},
		CredentialsByGroup: map[uint][]models.Credential{
			1: {{KeyHash: "h-a"}},
			2: {{KeyHash: "h-b"}},
		},
	}
}

// TestSelectGroup_CustomModelBypassesWhitelist 锁「自定义模型不受分组白名单约束」：
// 模型在 Provider.Config.custom_models 里时全部分组可选（改前：白名单不含 → 恒
// ErrNoGroupMatches，正是「自定义模型无组可服务」缺陷）；不在清单里的模型维持白名单口径。
func TestSelectGroup_CustomModelBypassesWhitelist(t *testing.T) {
	snap := customModelSnapshot()

	g, err := SelectGroup(&Selector{}, snap, "cm-1")
	if err != nil {
		t.Fatalf("SelectGroup(cm-1) error: %v（自定义模型必须可选中组）", err)
	}
	if g.Name != "组A" && g.Name != "组B" {
		t.Fatalf("SelectGroup(cm-1) = %q, want 组A/组B", g.Name)
	}

	// 非自定义且白名单不含：维持白名单过滤口径
	if _, err := SelectGroup(&Selector{}, snap, "not-declared"); !errors.Is(err, ErrNoGroupMatches) {
		t.Fatalf("SelectGroup(not-declared) err = %v, want %v", err, ErrNoGroupMatches)
	}
}

// TestGroupCandidatesForModel_CustomModelAllGroups 锁连通测试与生产选路同口径：
// 自定义模型的候选组 = 该供应商全部分组（管理端测试不得给出「生产走不到」的假结论）。
func TestGroupCandidatesForModel_CustomModelAllGroups(t *testing.T) {
	snap := customModelSnapshot()

	got := GroupCandidatesForModel(snap, "cm-1")
	if len(got) != 2 {
		t.Fatalf("候选数 = %d, want 2（自定义模型全部分组可服务）", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("顺序 = %v, want ID ASC [1 2]", []uint{got[0].ID, got[1].ID})
	}
}
