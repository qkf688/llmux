package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/balancer"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/channel"
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

// ── 提交后失败清钉（2026-10-01）──────────────────────────────────────────────
//
// 背景：清钉只发生在「已向客户端开写后上游断流」这一个出口（RecordLog 的 processer
// 失败分支是它的唯一收口）。提交前失败会换家重试并在成功后改钉，本来就自愈；502
// （多家全挂）走 handler 提前返回、根本不进 RecordLog，结构上到不了这里。
//
// 下面这一组锁的是**清与不清的判据**，四个面缺一不可：命中钉定才清、非断连才清、
// 落库失败不清（锁死「不得放外层收口」）、成功照旧改钉。

// depinFixture 铺好「一段对话已钉在某家」的前置态，并复位进程级共享粘性表。
// 钉定的是 depinPinnedMWP——「本次服务的就是它」的场景用它；服务方是别家时用
// depinOtherMWP。用具名常量而非裸字面量，避免把某个 case 的意图读错。
const (
	depinPinnedMWP uint = 43
	depinOtherMWP  uint = 99
)

func depinFixture(t *testing.T, key string) Before {
	t.Helper()
	balancer.DefaultAffinity().Reset()
	t.Cleanup(balancer.ResetDefaultAffinityForTest)
	SetSettingsReader(affinityGateStub{enabled: true})
	t.Cleanup(func() { SetSettingsReader(nil) })
	balancer.DefaultAffinity().Put(key, balancer.Target{ModelWithProviderID: depinPinnedMWP, CredentialID: 5}, time.Now())
	return Before{Model: "m", affinityKey: key}
}

func depinPinned(t *testing.T, key string) bool {
	t.Helper()
	_, ok := balancer.DefaultAffinity().Get(key, time.Now())
	return ok
}

func failingProcesser(err error) Processer {
	return func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		return nil, nil, err
	}
}

// TestRecordLog_PostCommitFailure_DepinMatrix 清钉判据矩阵。判据是**两条同时成立**：
// ① 本次服务的关联仍被这段对话钉着（比对在 balancer.DeleteIf）；② 失败不是客户端撤走。
// 前置态一律是「这段对话钉在 depinPinnedMWP」，差异全在 servingMWP 与错误形态上。
func TestRecordLog_PostCommitFailure_DepinMatrix(t *testing.T) {
	cases := []struct {
		name       string
		servingMWP uint
		procErr    error
		wantPinned bool
		why        string
	}{
		{
			name:       "服务的就是被钉的那家+上游腰斩→清",
			servingMWP: depinPinnedMWP,
			procErr:    fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantPinned: false,
			why:        "这次没能从钉定目标拿到成功且已无法换家，必须忘记它，否则每请求必撞一次",
		},
		{
			name:       "服务的是别家+上游腰斩→不清",
			servingMWP: depinOtherMWP,
			procErr:    fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantPinned: true,
			why:        "钉定指的仍是被记住的那家，它这次没参与——去清它属于误伤",
		},
		{
			name:       "服务的就是被钉的那家+客户端断连→不清",
			servingMWP: depinPinnedMWP,
			procErr:    fmt.Errorf("write response body failed: %w", context.Canceled),
			wantPinned: true,
			why:        "客户端自己关的连接，供应商没做错什么，记忆要留着",
		},
		{
			name:       "服务的就是被钉的那家+上游RST(读侧已打标)→清",
			servingMWP: depinPinnedMWP,
			procErr:    fmt.Errorf("write response body failed: %w", upstreamReadErr{err: rstLikeError()}),
			wantPinned: false,
			why:        "RST 是非超时 net.Error，isClientDisconnectErr 会误判成客户端断连；读侧打标优先于该反推（2026-10-01 code-review C2）",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)
			key := "k1"
			before := depinFixture(t, key)

			RecordLog(context.Background(), RecordLogInput{
				ReqStart:                   time.Now(),
				Reader:                     io.NopCloser(strings.NewReader("x")),
				Processer:                  failingProcesser(tc.procErr),
				LogID:                      0, // logID=0 时 updateChatLogByID 短路，本用例只验清钉判定
				Before:                     before,
				ProviderName:               "p",
				ServingModelWithProviderID: tc.servingMWP,
			})

			if got := depinPinned(t, key); got != tc.wantPinned {
				t.Fatalf("清钉后仍钉定 = %v, want %v（%s）", got, tc.wantPinned, tc.why)
			}
		})
	}
}

// rstLikeError 造一个形状与生产实测一致的上游读侧 RST：*net.OpError 且 Timeout()==false。
// 真机形态是 `wsarecv: ... forcibly closed by the remote host`（Windows）。用它而不直接
// 喂 io.ErrUnexpectedEOF，是为了让「类型反推会把 RST 误判成客户端断连」这条在测试里
// 可复现——生产日志 #13617 那次恰是 EOF 形态，恰好绕过了这个失效区间。
func rstLikeError() error {
	return &net.OpError{Op: "read", Net: "tcp", Err: errors.New("wsarecv: An existing connection was forcibly closed by the remote host")}
}

// TestRecordLog_PersistFailure_KeepsSticky 锁「清钉不得放外层收口」：processer 成功但
// 落库失败时 RecordLog 也会走到 :117 的外层留痕，那条 err 不是上游断流、也不是本次
// 尝试的失败——清钉若误挂在收口上，日志写库一失败就会把好端端的钉定抹掉。
// 制造手段：drop 掉 chat_logs 表让 persistChatLog 的更新失败（testutil 建的行在 drop 前）。
func TestRecordLog_PersistFailure_KeepsSticky(t *testing.T) {
	initChatRecordTestDB(t)
	before := depinFixture(t, "k1")
	logRow := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m", ProviderModel: "pm", ProviderName: "p", Status: "success"})

	procOK := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		return &models.ChatLog{Usage: models.Usage{TotalTokens: 1}}, &models.OutputUnion{OfString: "ok"}, nil
	}

	// 两次调用的入参完全相同，差别只在 DB 状态（通 / 不通），故收成闭包而非复制两遍
	// （重复率门：同块 10 行 × 2 处会报出新增重复块）。
	callRecordLog := func() {
		RecordLog(context.Background(), RecordLogInput{
			ReqStart:                   time.Now(),
			Reader:                     io.NopCloser(strings.NewReader("x")),
			Processer:                  procOK,
			LogID:                      logRow.ID,
			Before:                     before,
			ProviderName:               "p",
			ServingModelWithProviderID: depinPinnedMWP,
		})
	}

	callRecordLog()
	// RecordLog 的成功路径不参与改钉（改钉在 retry loop），此处只确认夹具的钉还在，
	// 后续用例才有对照基线。
	if !depinPinned(t, "k1") {
		t.Fatal("夹具前置条件不成立：调用前钉定应存在")
	}

	if err := models.DB.Exec("DROP TABLE chat_logs").Error; err != nil {
		t.Fatalf("drop chat_logs: %v", err)
	}

	callRecordLog()

	if !depinPinned(t, "k1") {
		t.Fatal("落库失败把钉定清掉了：清钉必须只挂在 processer 失败分支（响应已开写后的失败），不得放外层收口")
	}
}

// TestRecordLog_StickyDepin_LogsExactlyOnce 清钉必须留痕（自愈动作不可见 = 无法排查），
// 且**恰好一条**——与 RecordLog「一次失败恰一条留痕」的既有纪律同口径，多留会把一次
// 故障算成两次。
func TestRecordLog_StickyDepin_LogsExactlyOnce(t *testing.T) {
	initChatRecordTestDB(t)
	depinFixture(t, "k1")

	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:                   time.Now(),
		Reader:                     io.NopCloser(strings.NewReader("x")),
		Processer:                  failingProcesser(fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF)),
		LogID:                      0,
		Before:                     Before{Model: "m", affinityKey: "k1"},
		ProviderName:               "p",
		ServingModelWithProviderID: depinPinnedMWP,
	})

	ls := logs.String()
	if n := strings.Count(ls, "sticky target cleared"); n != 1 {
		t.Fatalf("清钉留痕 %d 条, want 1:\n%s", n, ls)
	}
	if !strings.Contains(ls, "k1") {
		t.Errorf("清钉留痕缺对话键（排查需要知道是哪段对话被清了）:\n%s", ls)
	}
}

// TestRecordLog_UpstreamRST_ClearsViaRealProcesser 端到端锁「上游 RST 也清钉」，且走**真实
// processer** 而非桩：判据正确性依赖「MarkUpstreamReader 的标记能穿过 process.go 的
// 错误上抛、穿过 RecordLog 的 %w 包装」这条跨文件契约——用桩 processer 测不出来，将来
// 谁把 process.go 的错误包成非 net 类型，桩测试照绿而生产行为已变。
func TestRecordLog_UpstreamRST_ClearsViaRealProcesser(t *testing.T) {
	initChatRecordTestDB(t)
	depinFixture(t, "k1")

	// 真实读侧：先吐一帧合法 SSE（越过提交门已验证的内容形态），再以 RST 结束。
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte("data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		_ = pw.CloseWithError(rstLikeError())
	}()

	RecordLog(context.Background(), RecordLogInput{
		ReqStart: time.Now(),
		// 与 handler/v1 的 attachLogRecord 同形：标记在**错误产生处**（读上游那一侧）
		// 打上，经 io.Copy 的返回错误 → CloseWithError → 日志管道 → processer 原样上抛
		// 一路传到判定层。NopCloser 只为满足 RecordLogInput.Reader 的接口形状
		//（生产里该字段是 io.Pipe 的读端，打标发生在 TeeReader 的源上，不经过这里）。
		Reader:                     io.NopCloser(MarkUpstreamReader(pr)),
		Processer:                  ProcesserOpenAI,
		LogID:                      0,
		Before:                     Before{Model: "m", Stream: true, affinityKey: "k1"},
		ProviderName:               "p",
		ServingModelWithProviderID: depinPinnedMWP,
	})

	if depinPinned(t, "k1") {
		t.Fatal("上游 RST 未清钉：该错误是非超时 net.Error，isClientDisconnectErr 会把它误判成" +
			"客户端断连——靠 MarkUpstreamReader 在错误产生处记下的方向标记纠正")
	}
}

// TestClearStickyTarget_Gate 锁清钉自身的门（与 RecordLog 侧的判定分开测）：空键与
// 开关关闭时本就不粘、无钉可清，必须早退返回 false 且不碰表；开关开启且钉定仍指向本次
// 服务的关联时返回 true。空键这条只能在此处断言——经 RecordLog 观察不到（空键在 Get 侧
// 恒为未钉，是 balancer 既有契约，见 affinity.go 的 Get 与 TestAffinity_PutGetAndEmptyKey）。
func TestClearStickyTarget_Gate(t *testing.T) {
	balancer.DefaultAffinity().Reset()
	t.Cleanup(balancer.ResetDefaultAffinityForTest)

	SetSettingsReader(affinityGateStub{enabled: true})
	t.Cleanup(func() { SetSettingsReader(nil) })
	balancer.DefaultAffinity().Put("k1", balancer.Target{ModelWithProviderID: depinPinnedMWP, CredentialID: 5}, time.Now())

	if clearStickyTarget(context.Background(), Before{Model: "m", affinityKey: ""}, depinPinnedMWP) {
		t.Fatal("空键清钉 = true，want false（未识别对话身份本就不粘）")
	}
	if !depinPinned(t, "k1") {
		t.Fatal("空键清钉动了表：早退契约被破坏")
	}

	SetSettingsReader(affinityGateStub{enabled: false})
	if clearStickyTarget(context.Background(), Before{Model: "m", affinityKey: "k1"}, depinPinnedMWP) {
		t.Fatal("开关关闭时清钉 = true，want false（不粘即无钉可清）")
	}
	if !depinPinned(t, "k1") {
		t.Fatal("开关关闭时清钉动了表：早退契约被破坏")
	}

	SetSettingsReader(affinityGateStub{enabled: true})
	if clearStickyTarget(context.Background(), Before{Model: "m", affinityKey: "k1"}, depinOtherMWP) {
		t.Fatal("服务的是别家时清钉 = true，want false（比对不符，别人的钉不能删）")
	}
	if !depinPinned(t, "k1") {
		t.Fatal("比对不符却动了表：DeleteIf 的比对契约被破坏")
	}
	if !clearStickyTarget(context.Background(), Before{Model: "m", affinityKey: "k1"}, depinPinnedMWP) {
		t.Fatal("开关开启且比对相符：清钉 = false，want true")
	}
	if depinPinned(t, "k1") {
		t.Fatal("清钉后仍钉定")
	}
	if clearStickyTarget(context.Background(), Before{Model: "m", affinityKey: "k1"}, depinPinnedMWP) {
		t.Fatal("重复清钉 = true，want false（已无钉）")
	}
}

// TestBalanceChat_ServingMwpID 锁 ServingModelWithProviderID 的**语义**面（不是字段搬运
// 的琐碎断言）：它必须报「本次真正服务的那家」。第三个 case 是 2026-10-01 code-review
// C1 的回归锁——早期版本在这里报的是「读钉时看到的那家」，于是「钉定目标提交前失败 →
// 转移成功 → 赢家随后腰斩」这个最常见的循环被挡在清钉之外，而所有测试照绿。
//
// Before 走真实 BeforerOpenAI 而非手搓：生产路径就是把 Beforer 的产物原样传下去，
// 手搓的 Before 不带 affinityKey 会让粘性整体不生效，测不到东西。
func TestBalanceChat_ServingMwpID(t *testing.T) {
	cases := []struct {
		name string
		// pinToGood=true 时把该对话钉到「好候选」；pinToDead=true 时钉到会失败的候选。
		pinToGood bool
		pinToDead bool
		// wantGood=true 期望服务方是好候选；false 期望是那个连接被拒的失败候选。
		wantGood bool
		why      string
	}{
		{name: "钉定命中", pinToGood: true, wantGood: true, why: "粘性直返拿到的就是被记住的那家"},
		{name: "钉定目标失败后转移成功", pinToDead: true, wantGood: true, why: "转移后的赢家才是服务方——改钉也刚指向它，故它随后腰斩必须能清"},
		{name: "钉定目标失败且无替补", pinToDead: true, wantGood: false, why: "全候选失败时整体 502，不产出结果（由下面 TestBalanceChat_ExhaustedKeepsSticky 单独锁）"},
	}

	for _, tc := range cases {
		if tc.name == "钉定目标失败且无替补" {
			continue // 该场景不该走成功路径，由穷尽用例覆盖
		}
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)
			cipher := setupChatChannelCipher(t)
			channel.ResetDefaultSelectorForTest()
			t.Cleanup(channel.ResetDefaultSelectorForTest)
			balancer.DefaultAffinity().Reset()
			t.Cleanup(balancer.ResetDefaultAffinityForTest)

			const goodSSE = "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n" +
				"data: [DONE]\n\n"
			good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(goodSSE))
			}))
			t.Cleanup(good.Close)

			model := models.Model{Name: "affhit-" + tc.name}
			if err := models.DB.Create(&model).Error; err != nil {
				t.Fatalf("create model: %v", err)
			}
			goodMwp := setupGateFailoverProvider(t, cipher, model.ID, "good", good.URL, 1)

			// 会失败的候选：端点指向无人监听的端口，连接被拒（毫秒级，不拖测试）。
			var deadMwp models.ModelWithProvider
			if tc.pinToDead {
				deadMwp = setupGateFailoverProvider(t, cipher, model.ID, "dead", "http://127.0.0.1:1", 2)
			}

			raw := []byte(`{"model":"` + model.Name + `","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
			before, err := BeforerOpenAI(raw)
			if err != nil {
				t.Fatalf("BeforerOpenAI: %v", err)
			}

			pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{goodMwp, deadMwp})
			if err != nil {
				t.Fatalf("buildCandidatePool: %v", err)
			}

			switch {
			case tc.pinToGood:
				balancer.DefaultAffinity().Put(before.affinityKey, balancer.Target{ModelWithProviderID: goodMwp.ID}, time.Now())
			case tc.pinToDead:
				balancer.DefaultAffinity().Put(before.affinityKey, balancer.Target{ModelWithProviderID: deadMwp.ID}, time.Now())
			}

			result, err := BalanceChat(context.Background(), BalanceInput{
				Start:  time.Now(),
				Style:  string(consts.StyleOpenAI),
				Before: *before,
				ProvidersWithMeta: ProvidersWithMeta{
					CandidatePool: pool,
					Model:         &model,
				},
				ReqMeta: models.ReqMeta{Header: http.Header{}},
			})
			if err != nil {
				t.Fatalf("BalanceChat err = %v, want 成功（转移后应由好候选兜住）", err)
			}
			defer result.Response.Body.Close()
			if _, err := io.ReadAll(result.Response.Body); err != nil {
				t.Fatalf("read body: %v", err)
			}

			if tc.wantGood && result.ServingModelWithProviderID != goodMwp.ID {
				t.Fatalf("ServingModelWithProviderID = %d, want 好候选 %d（%s）", result.ServingModelWithProviderID, goodMwp.ID, tc.why)
			}
		})
	}
}

// TestBalanceChat_ExhaustedKeepsSticky 锁「候选穷尽（502）不清钉」。当前靠 handler 的
// 控制流保证（BalanceChat 返回 err 时根本不进 RecordLog），本用例把它钉成机械门：若
// 日后有人把清钉上移到 retry loop / BalanceChat 这类「看起来更自然」的位置，穷尽路径会
// 开始清钉，而这一条会转红。
func TestBalanceChat_ExhaustedKeepsSticky(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	balancer.DefaultAffinity().Reset()
	t.Cleanup(balancer.ResetDefaultAffinityForTest)

	model := models.Model{Name: "aff-exhaust"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	// 唯一的候选指向无人监听的端口：连接被拒 → 提交前失败 → 穷尽 → 502。
	deadMwp := setupGateFailoverProvider(t, cipher, model.ID, "dead", "http://127.0.0.1:1", 1)

	raw := []byte(`{"model":"aff-exhaust","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	before, err := BeforerOpenAI(raw)
	if err != nil {
		t.Fatalf("BeforerOpenAI: %v", err)
	}
	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{deadMwp})
	if err != nil {
		t.Fatalf("buildCandidatePool: %v", err)
	}
	balancer.DefaultAffinity().Put(before.affinityKey, balancer.Target{ModelWithProviderID: deadMwp.ID}, time.Now())

	_, err = BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  string(consts.StyleOpenAI),
		Before: *before,
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &model,
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err == nil {
		t.Fatal("BalanceChat err = nil, want 穷尽错误（502）")
	}
	if depinPinned(t, before.affinityKey) == false {
		t.Fatal("候选穷尽把钉定清掉了：多家全挂时不清钉（否则全体活跃会话会被一次全挂惊群清空）")
	}
}

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
