package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
	"github.com/qkf688/llmux/service/virtualmodel"
)

// TestBeforeAffinityKey_StableAcrossSerialization 锁 AC-7：对话身份键取语义值——
// 字段顺序不同、注入 stream_options、多轮追加消息（尾部增长）都不得改变同对话的键；
// system/首条消息语义变化 = 新对话，键必须不同。
func TestBeforeAffinityKey_StableAcrossSerialization(t *testing.T) {
	mustBefore := func(style string, raw string) Before {
		t.Helper()
		var (
			b   *Before
			err error
		)
		switch style {
		case "openai":
			b, err = BeforerOpenAI([]byte(raw))
		case "openai-res":
			b, err = BeforerOpenAIRes([]byte(raw))
		case "anthropic":
			b, err = BeforerAnthropic([]byte(raw))
		}
		if err != nil {
			t.Fatalf("Beforer(%s) error: %v", style, err)
		}
		if b.affinityKey == "" {
			t.Fatalf("Beforer(%s) affinityKey 为空", style)
		}
		return *b
	}

	base := mustBefore("openai", `{"model":"m","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}]}`)

	reordered := mustBefore("openai", `{"messages":[{"content":"sys","role":"system"},{"content":"hi","role":"user"}],"model":"m"}`)
	if reordered.affinityKey != base.affinityKey {
		t.Fatal("字段顺序不同导致键漂移（AC-7：语义值同则键同）")
	}

	turn2 := mustBefore("openai", `{"model":"m","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`)
	if turn2.affinityKey != base.affinityKey {
		t.Fatal("多轮追加消息导致键漂移（头部不变 = 同一对话）")
	}

	// 注入 stream_options（BeforerOpenAI 对 stream:true 注入）不得改变键。
	withoutInjection := mustBefore("openai", `{"model":"m","stream":true,"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}]}`)
	withInjection := mustBefore("openai", `{"model":"m","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}]}`)
	if withoutInjection.affinityKey != withInjection.affinityKey {
		t.Fatal("stream_options 注入导致键漂移")
	}

	otherSystem := mustBefore("openai", `{"model":"m","messages":[{"role":"system","content":"OTHER"},{"role":"user","content":"hi"}]}`)
	if otherSystem.affinityKey == base.affinityKey {
		t.Fatal("system 语义变化必须视为新对话（键不同）")
	}
	otherFirstMsg := mustBefore("openai", `{"model":"m","messages":[{"role":"system","content":"sys"},{"role":"user","content":"bye"}]}`)
	if otherFirstMsg.affinityKey == base.affinityKey {
		t.Fatal("首条消息语义变化必须视为新对话（键不同）")
	}

	// anthropic：system 是顶层键，多轮追加同样不漂移。
	anBase := mustBefore("anthropic", `{"model":"m","system":"sys","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)
	anTurn2 := mustBefore("anthropic", `{"model":"m","system":"sys","max_tokens":64,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`)
	if anTurn2.affinityKey != anBase.affinityKey {
		t.Fatal("anthropic 多轮追加导致键漂移")
	}
	if anBase.affinityKey == base.affinityKey {
		t.Fatal("anthropic/openai 不同对话（首条消息形状不同）键撞车")
	}

	// openai-res：input 数组首项参与身份；字符串 input 用其内容参与（不得恒同键）。
	resA := mustBefore("openai-res", `{"model":"m","input":"hi"}`)
	resB := mustBefore("openai-res", `{"model":"m","input":"yo"}`)
	if resA.affinityKey == resB.affinityKey {
		t.Fatal("字符串 input 内容不同必须键不同（否则不同对话互钉）")
	}
}

// TestBeforeAffinityKey_GatewayPatchesDoNotDrift 锁 AC-7 的网关侧注入面：BeforerOpenAI 的
// FillMissingToolCallIDs 会为缺 id 的 tool_calls 生成**随机** id——身份键必须取自网关注入
// 前的请求语义，否则同一请求每次键不同、粘性永不命中且向粘性表灌垃圾键。
func TestBeforeAffinityKey_GatewayPatchesDoNotDrift(t *testing.T) {
	raw := `{"model":"m","messages":[{"role":"system","content":"sys"},{"role":"assistant","tool_calls":[{"function":{"name":"f","arguments":"{}"}}]}]}`

	a, err := BeforerOpenAI([]byte(raw))
	if err != nil {
		t.Fatalf("BeforerOpenAI #1: %v", err)
	}
	b, err := BeforerOpenAI([]byte(raw))
	if err != nil {
		t.Fatalf("BeforerOpenAI #2: %v", err)
	}
	if a.affinityKey != b.affinityKey {
		t.Fatalf("网关注入（随机 tool_call id）导致键漂移：%q vs %q", a.affinityKey, b.affinityKey)
	}
}

// TestPreferRealModelsFirst 锁 AC-4 的前移语义（含 Ruling 决策）：钉定模型稳定前移、
// 其余相对顺序不变、未钉/不在列表/已在首位原序返回。
//
// Ruling（2026-09-23 审查回批）：虚拟层前移**不做 mapping Priority 分档**（与候选层
// 「粘性不越权优先级档」刻意不对称）——OrderedRealModel 列表是故障转移序列而非并行
// 分摊策略，前移只改「先试哪个」、失败照常按原序转移，钉定只来自上一次真实成功。
// 代价：管理员调高他人 mapping priority 后，活跃对话仍钉旧模型直到闲置过期（30min TTL）。
// 本用例把「低 Priority 也前移」钉死，防行为被按候选层原则「顺手修正」。
func TestPreferRealModelsFirst(t *testing.T) {
	mk := func(id uint, priority int) virtualmodel.OrderedRealModel {
		return virtualmodel.OrderedRealModel{Model: models.Model{ID: id}, Priority: priority}
	}
	// [1(P10), 2(P5), 3(P5)]：刻意含不同 Priority。
	list := []virtualmodel.OrderedRealModel{mk(1, 10), mk(2, 5), mk(3, 5)}

	// 钉定非首位 → 稳定前移，其余相对顺序不变（3 仍在 2 后）。
	got := preferRealModelsFirst(list, 2)
	if got[0].Model.ID != 2 || got[1].Model.ID != 1 || got[2].Model.ID != 3 {
		t.Fatalf("前移结果 = %v, want [2 1 3]", []uint{got[0].Model.ID, got[1].Model.ID, got[2].Model.ID})
	}

	// Ruling：低 Priority 的钉定照样前移（虚拟层不做档约束）。
	got = preferRealModelsFirst(list, 3)
	if got[0].Model.ID != 3 || got[1].Model.ID != 1 || got[2].Model.ID != 2 {
		t.Fatalf("低 Priority 钉定未前移 = %v, want [3 1 2]（Ruling：虚拟层不做档约束）", []uint{got[0].Model.ID, got[1].Model.ID, got[2].Model.ID})
	}

	// 钉定不在列表 / 未钉 / 已在首位 → 原序。
	for name, pinned := range map[string]uint{"不在列表": 99, "未钉": 0, "已在首位": 1} {
		got = preferRealModelsFirst(list, pinned)
		for i, id := range []uint{1, 2, 3} {
			if got[i].Model.ID != id {
				t.Fatalf("%s: 顺序 = %v, want [1 2 3]（原序返回）", name, []uint{got[0].Model.ID, got[1].Model.ID, got[2].Model.ID})
			}
		}
	}

	// 原切片不被污染（返回副本时入参保持原序）。
	if list[0].Model.ID != 1 {
		t.Fatal("preferRealModelsFirst 污染了入参切片")
	}
}

// affinityGateStub 只认缓存亲和键的 Reader 桩：其余键回默认值。
type affinityGateStub struct {
	enabled bool
}

func (s affinityGateStub) Bool(_ context.Context, key string, defaultValue bool) bool {
	if key == models.SettingKeyCacheAffinityEnabled {
		return s.enabled
	}
	return defaultValue
}
func (s affinityGateStub) Int(_ context.Context, _ string, defaultValue, _ int) int {
	return defaultValue
}
func (s affinityGateStub) String(_ context.Context, _ string, defaultValue string) string {
	return defaultValue
}
func (s affinityGateStub) BoolE(ctx context.Context, key string, defaultValue bool) (bool, error) {
	return s.Bool(ctx, key, defaultValue), nil
}
func (s affinityGateStub) IntE(ctx context.Context, key string, defaultValue, minValue int) (int, error) {
	return s.Int(ctx, key, defaultValue, minValue), nil
}

var _ settings.Reader = affinityGateStub{}

// TestAffinityKeyFor_Gate 锁 AC-5 的键侧门：设置关闭或键为空时返回 ""（粘性统一
// 「关闭」信号，balancer 表对空键恒视为未钉）；开启且有键时原样返回。
func TestAffinityKeyFor_Gate(t *testing.T) {
	before := Before{Model: "m", affinityKey: "k1"}

	SetSettingsReader(affinityGateStub{enabled: true})
	t.Cleanup(func() { SetSettingsReader(nil) })
	if got := affinityKeyFor(context.Background(), before); got != "k1" {
		t.Fatalf("开启时 affinityKeyFor = %q, want k1", got)
	}

	SetSettingsReader(affinityGateStub{enabled: false})
	if got := affinityKeyFor(context.Background(), before); got != "" {
		t.Fatalf("关闭时 affinityKeyFor = %q, want \"\"（AC-5：关闭=完全不粘）", got)
	}

	SetSettingsReader(affinityGateStub{enabled: true})
	if got := affinityKeyFor(context.Background(), Before{Model: "m"}); got != "" {
		t.Fatalf("空键 affinityKeyFor = %q, want \"\"（未识别对话身份不粘）", got)
	}
}
