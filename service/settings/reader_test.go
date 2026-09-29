package settings

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// readSkipHarness 把「只收本包留痕的日志器 + 可推进的时钟」装到一处：
// 限频是时间行为，用例必须能推时间而不是真等一个窗口（否则测试要跑 60 秒）。
type readSkipHarness struct {
	logs *strings.Builder
	now  time.Time
}

func newReadSkipHarness(t *testing.T) *readSkipHarness {
	t.Helper()
	resetReadSkipLimiter()

	h := &readSkipHarness{
		logs: &strings.Builder{},
		now:  time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}
	skipLogNow = func() time.Time { return h.now }

	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(h.logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		skipLogNow = time.Now
		resetReadSkipLimiter()
	})

	return h
}

func (h *readSkipHarness) advance(d time.Duration) { h.now = h.now.Add(d) }
func (h *readSkipHarness) lines() int {
	return strings.Count(h.logs.String(), "dependent write step skipped")
}

// TestLogReadSkip_SameKeyBurst_LogsOncePerWindow 留痕放大系数契约（Stage 36）：
// 设置读失败期一次请求会走过多个「读配置→写库」步骤，逐步留痕会让日志量 ≈ 请求量 × 步数，
// 恰在排障最需要日志时把日志淹掉。同一个设置项在窗口内只出第一条，其余压掉并计数。
func TestLogReadSkip_SameKeyBurst_LogsOncePerWindow(t *testing.T) {
	h := newReadSkipHarness(t)

	for i := 0; i < 4; i++ {
		h.advance(time.Second)
		LogReadSkip("weight decay", "auto_weight_decay", errors.New("db locked"))
	}

	if n := h.lines(); n != 1 {
		t.Errorf("同一设置项连打 4 次留痕写出 %d 条，want 1（放大系数必须与请求量解耦）:\n%s", n, h.logs.String())
	}
}

// TestLogReadSkip_WindowElapsed_LogsAgainWithSuppressedCount 窗口边界与信息不丢：
// 窗口过去后必须再出留痕（否则故障持续期事后一片空白），且把窗口内压掉的条数带出来。
func TestLogReadSkip_WindowElapsed_LogsAgainWithSuppressedCount(t *testing.T) {
	h := newReadSkipHarness(t)

	LogReadSkip("weight decay", "auto_weight_decay", errors.New("db locked"))

	for i := 0; i < 3; i++ {
		h.advance(time.Second)
		LogReadSkip("weight decay", "auto_weight_decay", errors.New("db locked"))
	}
	if n := h.lines(); n != 1 {
		t.Fatalf("窗口内留痕 %d 条，want 1:\n%s", n, h.logs.String())
	}

	h.advance(skipLogWindow)
	LogReadSkip("weight decay", "auto_weight_decay", errors.New("db locked"))

	if n := h.lines(); n != 2 {
		t.Fatalf("窗口过后留痕共 %d 条，want 2（窗口内被压的必须在下一条带出）:\n%s", n, h.logs.String())
	}
	last := h.logs.String()[strings.LastIndex(h.logs.String(), "level=WARN"):]
	if !strings.Contains(last, "suppressed=3") {
		t.Errorf("第二条留痕未带出被压条数（want suppressed=3）:\n%s", last)
	}
}

// TestLogReadSkip_DistinctKeys_DoNotShareWindow 限频按设置项独立：
// 库不稳时通常是所有键一起读失败，共用一个窗口会把其它键的首次故障一起吞掉。
func TestLogReadSkip_DistinctKeys_DoNotShareWindow(t *testing.T) {
	h := newReadSkipHarness(t)

	LogReadSkip("weight decay", "auto_weight_decay", errors.New("db locked"))
	LogReadSkip("priority decay", "auto_priority_decay", errors.New("db locked"))

	if n := h.lines(); n != 2 {
		t.Errorf("两个不同设置项各出一条，共 %d 条，want 2（限频窗口必须按设置项独立）:\n%s", n, h.logs.String())
	}
}
