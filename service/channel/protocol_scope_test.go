package channel

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// scopeProvider 构造测试用供应商类型（type 决定主格式）。
func scopeSnapshot(providerType string, eps []models.Endpoint, mp *models.ModelWithProvider) *Snapshot {
	return &Snapshot{
		Provider:    models.Provider{Model: gorm.Model{ID: 7}, Type: providerType},
		Endpoints:   eps,
		Association: mp,
	}
}

func protocolsPtr(list ...string) *[]string { return &list }
func protocolPtr(p string) *string          { return &p }

// TestResolveProtocolScope_InheritKeepsTodayRules 锁 AC-1：关联两项都为空时，协议集合 =
// 供应商已启用端点集，主格式 = ProtocolOfType(Provider.Type)，不做任何兜底。
func TestResolveProtocolScope_InheritKeepsTodayRules(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolAnthropic), Enabled: true},
		{Model: gorm.Model{ID: 2}, Protocol: string(consts.ProtocolOpenAI), Enabled: false},
		{Model: gorm.Model{ID: 3}, Protocol: string(consts.ProtocolResponses), Enabled: true},
	}

	scope, err := ResolveProtocolScope("openai", eps, nil)
	if err != nil {
		t.Fatalf("ResolveProtocolScope: %v", err)
	}
	if got := scope.Allowed; len(got) != 2 || got[0] != consts.ProtocolResponses || got[1] != consts.ProtocolAnthropic {
		t.Fatalf("Allowed = %v, want [responses anthropic]（按固定先后排序，停用端点不入集合）", got)
	}
	if scope.Primary != consts.ProtocolOpenAI {
		t.Fatalf("Primary = %v, want openai（继承 = 今天的兜底，端点被停用也不改选其他协议）", scope.Primary)
	}
	if len(scope.Explicit) != 0 {
		t.Fatalf("Explicit = %v, want 空（继承态不越过启停）", scope.Explicit)
	}
}

// TestResolveProtocolScope_OverridePrimaryNotInSetFallsBack 锁「只设集合」的规则：
// 供应商主格式不在集合内 → 集合内固定先后第一条。
func TestResolveProtocolScope_OverridePrimaryNotInSetFallsBack(t *testing.T) {
	mp := &models.ModelWithProvider{
		Protocols: protocolsPtr(string(consts.ProtocolAnthropic), string(consts.ProtocolResponses)),
	}
	scope, err := ResolveProtocolScope("openai", nil, mp)
	if err != nil {
		t.Fatalf("ResolveProtocolScope: %v", err)
	}
	if got := scope.Allowed; len(got) != 2 || got[0] != consts.ProtocolResponses || got[1] != consts.ProtocolAnthropic {
		t.Fatalf("Allowed = %v, want [responses anthropic]", got)
	}
	if scope.Primary != consts.ProtocolResponses {
		t.Fatalf("Primary = %v, want responses（集合内固定先后第一条）", scope.Primary)
	}
	if !scope.Explicit[consts.ProtocolAnthropic] || !scope.Explicit[consts.ProtocolResponses] {
		t.Fatalf("Explicit = %v, want 两条都标记显式", scope.Explicit)
	}
}

// TestResolveProtocolScope_ExplicitPrimaryWins 锁主格式覆盖优先于供应商类型。
func TestResolveProtocolScope_ExplicitPrimaryWins(t *testing.T) {
	mp := &models.ModelWithProvider{
		Protocols:       protocolsPtr(string(consts.ProtocolOpenAI), string(consts.ProtocolAnthropic)),
		PrimaryProtocol: protocolPtr(string(consts.ProtocolAnthropic)),
	}
	scope, err := ResolveProtocolScope("openai", nil, mp)
	if err != nil {
		t.Fatalf("ResolveProtocolScope: %v", err)
	}
	if scope.Primary != consts.ProtocolAnthropic {
		t.Fatalf("Primary = %v, want anthropic（显式覆盖优先）", scope.Primary)
	}
}

// TestResolveProtocolScope_InvalidOverrideFails 锁非法配置不被静默回落：
// 主格式不在集合内、集合为空、未知协议都返回错误。
func TestResolveProtocolScope_InvalidOverrideFails(t *testing.T) {
	cases := []struct {
		name string
		mp   *models.ModelWithProvider
	}{
		{"empty set", &models.ModelWithProvider{Protocols: protocolsPtr()}},
		{"primary not in set", &models.ModelWithProvider{
			Protocols:       protocolsPtr(string(consts.ProtocolOpenAI)),
			PrimaryProtocol: protocolPtr(string(consts.ProtocolAnthropic)),
		}},
		{"unknown protocol", &models.ModelWithProvider{Protocols: protocolsPtr("mystery")}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ResolveProtocolScope("openai", nil, tt.mp); !errors.Is(err, ErrEndpointUnavailable) {
				t.Fatalf("err = %v, want ErrEndpointUnavailable", err)
			}
		})
	}
}

// TestEndpointOfProtocol_DetectionMatchesRuntimeForDisabledEndpoint 锁「检测与运行同源」：
// 端点已配但被停用时，协议检测（includeDisabled=true）与关联显式勾选的运行侧必须取同一条 URL。
// 不同源的后果是「检测说可用 → 一键写入 → 生产流量打到一条从未被测过的 URL」。
func TestEndpointOfProtocol_DetectionMatchesRuntimeForDisabledEndpoint(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolAnthropic), URL: "https://disabled.example/v1", Enabled: false},
	}
	detected, ok := EndpointOfProtocol(eps, consts.ProtocolAnthropic, true)
	if !ok || detected.URL != "https://disabled.example/v1" {
		t.Fatalf("detection endpoint = %+v (ok=%v), want 停用端点的配置 URL", detected, ok)
	}

	scope, err := ResolveProtocolScope("openai", eps, &models.ModelWithProvider{
		Protocols: protocolsPtr(string(consts.ProtocolAnthropic)),
	})
	if err != nil {
		t.Fatalf("ResolveProtocolScope: %v", err)
	}
	runtimeEP, ok := scope.EndpointFor(eps, consts.ProtocolAnthropic)
	if !ok {
		t.Fatal("运行侧必须选中被覆盖的停用端点")
	}
	if runtimeEP.URL != detected.URL {
		t.Fatalf("检测 URL %q 与运行 URL %q 必须同源", detected.URL, runtimeEP.URL)
	}
}

// TestSelectEndpoint_AssociationNarrowsProtocol 锁 AC-2 / AC-3：同一供应商下，
// A 关联只允许 Anthropic、B 未设置；同一入站 OpenAI 格式时 A 走转换（选中 anthropic），
// B 仍直通 openai。删掉「关联集合优先」后本用例必红。
func TestSelectEndpoint_AssociationNarrowsProtocol(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolOpenAI), Enabled: true},
		{Model: gorm.Model{ID: 2}, Protocol: string(consts.ProtocolAnthropic), Enabled: true},
	}

	inherited := scopeSnapshot("openai", eps, nil)
	epB, err := SelectEndpoint(inherited, consts.FormatOpenAIChat)
	if err != nil {
		t.Fatalf("B (inherit) SelectEndpoint: %v", err)
	}
	if epB.ID != 1 {
		t.Fatalf("B 选中端点 %d, want 1（继承 = 入站 openai 直通）", epB.ID)
	}

	narrowed := scopeSnapshot("openai", eps, &models.ModelWithProvider{
		Protocols: protocolsPtr(string(consts.ProtocolAnthropic)),
	})
	epA, err := SelectEndpoint(narrowed, consts.FormatOpenAIChat)
	if err != nil {
		t.Fatalf("A (anthropic only) SelectEndpoint: %v", err)
	}
	if epA.ID != 2 {
		t.Fatalf("A 选中端点 %d, want 2（集合内无 openai → 转成主格式 anthropic）", epA.ID)
	}
}

// TestSelectEndpoint_AssociationOverridesProviderDisable 锁 AC-4：关联显式勾选的协议
// 越过供应商的端点启停。
func TestSelectEndpoint_AssociationOverridesProviderDisable(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolOpenAI), Enabled: true},
		{Model: gorm.Model{ID: 2}, Protocol: string(consts.ProtocolAnthropic), Enabled: false},
	}
	snap := scopeSnapshot("openai", eps, &models.ModelWithProvider{
		Protocols: protocolsPtr(string(consts.ProtocolAnthropic)),
	})
	ep, err := SelectEndpoint(snap, consts.FormatAnthropic)
	if err != nil {
		t.Fatalf("SelectEndpoint: %v", err)
	}
	if ep.ID != 2 || ep.Enabled {
		t.Fatalf("选中端点 = %+v, want 停用的 anthropic 端点 ID 2（关联覆盖优先于供应商启停）", ep)
	}
}

// TestSelectEndpoint_AssociationSynthesizesMissingEndpoint 锁 AC-5：关联勾了供应商
// 没配的协议 → 合成空 URL 端点，由 BuildConfig 回落 base_url。
func TestSelectEndpoint_AssociationSynthesizesMissingEndpoint(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolOpenAI), Enabled: true},
	}
	snap := scopeSnapshot("openai", eps, &models.ModelWithProvider{
		Protocols: protocolsPtr(string(consts.ProtocolAnthropic)),
	})
	ep, err := SelectEndpoint(snap, consts.FormatOpenAIChat)
	if err != nil {
		t.Fatalf("SelectEndpoint: %v", err)
	}
	if ep.Protocol != string(consts.ProtocolAnthropic) || ep.URL != "" || ep.ID != 0 {
		t.Fatalf("合成端点 = %+v, want 协议 anthropic / 空 URL / ID 0（走 provider base_url）", ep)
	}
}

// TestSelectEndpoint_InheritPrimaryDisabledStillFails 锁 AC-1 的失败面：继承态下供应商
// 主格式端点被停用，不得改用别的协议兜底（与改动前 ErrEndpointUnavailable 一致）。
func TestSelectEndpoint_InheritPrimaryDisabledStillFails(t *testing.T) {
	eps := []models.Endpoint{
		{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolOpenAI), Enabled: false},
		{Model: gorm.Model{ID: 2}, Protocol: string(consts.ProtocolAnthropic), Enabled: true},
	}
	snap := scopeSnapshot("openai", eps, nil)
	if _, err := SelectEndpoint(snap, consts.FormatOpenAIChat); !errors.Is(err, ErrEndpointUnavailable) {
		t.Fatalf("err = %v, want ErrEndpointUnavailable", err)
	}
}
