package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
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
