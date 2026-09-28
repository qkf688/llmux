package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// Stage 23 回归：回复日志（ResponseBody/ChatIO 累积）超上限截头尾——
// 流式与非流式同一把刀（common.TruncateHeadTailUTF8），上限 getResponseLogMaxBytes。

// responseCapSettingsReader 测试用 settings.Reader stub：只覆写回复日志上限键，
// 其余回退 defaultValue（与 credCooldownSettingsReader 同模式）。
type responseCapSettingsReader struct {
	MaxKiB int
}

func (r responseCapSettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (r responseCapSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	if key == models.SettingKeyResponseLogMaxKiB {
		return r.MaxKiB
	}
	return def
}

func (r responseCapSettingsReader) String(_ context.Context, _ string, def string) string {
	return def
}

func (r responseCapSettingsReader) BoolE(ctx context.Context, key string, def bool) (bool, error) {
	return r.Bool(ctx, key, def), nil
}

func (r responseCapSettingsReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	return r.Int(ctx, key, def, min), nil
}

var _ settings.Reader = responseCapSettingsReader{}

func setResponseLogMaxKiB(t *testing.T, kib int) {
	t.Helper()
	SetSettingsReader(responseCapSettingsReader{MaxKiB: kib})
	t.Cleanup(func() { SetSettingsReader(nil) })
}

// TestApplyResponseBodyToLog_ResponseBodyCappedHeadTail 非流式 OfString 超限存头尾+标记。
func TestApplyResponseBodyToLog_ResponseBodyCappedHeadTail(t *testing.T) {
	initChatRecordTestDB(t)
	setResponseLogMaxKiB(t, 16) // 16 KiB

	payload := strings.Repeat("字", (64<<10)/3) // 64 KiB 中文，截断处多在多字节中间
	log := &models.ChatLog{}
	applyResponseBodyToLog(context.Background(), log, &models.OutputUnion{OfString: payload})

	limit := getResponseLogMaxBytes(context.Background())
	got := log.ResponseBody
	if len(got) > limit {
		t.Fatalf("落库 %d 字节, want ≤ %d（上限内含标记）", len(got), limit)
	}
	if !strings.Contains(got, "内容过长已截断") {
		t.Fatalf("超限必须带统一头尾截断标记")
	}
	// 头段是原文前缀、尾段是原文后缀（rune 边界内取 12 字节核对）
	if !strings.HasPrefix(payload, got[:12]) {
		t.Fatalf("头段不是原文前缀")
	}
	if !strings.HasSuffix(payload, got[len(got)-12:]) {
		t.Fatalf("尾段不是原文后缀")
	}
}

// TestApplyResponseBodyToLog_StreamChunksCapped 流式 OfStringArray 拼整篇超限同口径。
func TestApplyResponseBodyToLog_StreamChunksCapped(t *testing.T) {
	initChatRecordTestDB(t)
	setResponseLogMaxKiB(t, 16)

	chunk := strings.Repeat("a", 8<<10)
	chunks := make([]string, 0, 20)
	for i := 0; i < 20; i++ { // 20 × 8KiB = 160KiB >> 16KiB 上限
		chunks = append(chunks, chunk)
	}
	log := &models.ChatLog{}
	applyResponseBodyToLog(context.Background(), log, &models.OutputUnion{OfStringArray: chunks})

	limit := getResponseLogMaxBytes(context.Background())
	if len(log.ResponseBody) > limit {
		t.Fatalf("流式落库 %d 字节, want ≤ %d（此前无上限，必须收口）", len(log.ResponseBody), limit)
	}
	if !strings.Contains(log.ResponseBody, "内容过长已截断") {
		t.Fatalf("流式超限必须带头尾截断标记")
	}
}

// TestApplyResponseBodyToLog_WithinLimitUnchanged 未超限原样完整。
func TestApplyResponseBodyToLog_WithinLimitUnchanged(t *testing.T) {
	initChatRecordTestDB(t)
	setResponseLogMaxKiB(t, 64)

	payload := `{"ok":true}`
	log := &models.ChatLog{}
	applyResponseBodyToLog(context.Background(), log, &models.OutputUnion{OfString: payload})
	if log.ResponseBody != payload {
		t.Fatalf("上限内必须原样: %q", log.ResponseBody)
	}
}

// TestPersistChatLog_ChatIOOutputCapped ChatIO 的流式 chunk 数组同受预算约束。
func TestPersistChatLog_ChatIOOutputCapped(t *testing.T) {
	initChatRecordTestDB(t)
	setResponseLogMaxKiB(t, 16)

	chunk := strings.Repeat("b", 8<<10)
	chunks := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		chunks = append(chunks, chunk)
	}
	logRow := models.ChatLog{Status: "success"}
	if err := models.DB.Create(&logRow).Error; err != nil {
		t.Fatalf("create log row: %v", err)
	}
	in := RecordLogInput{LogID: logRow.ID, IOLog: true}
	output := &models.OutputUnion{OfStringArray: chunks}

	if err := persistChatLog(context.Background(), in, models.ChatLog{}, output, models.RawLogOptions{}); err != nil {
		t.Fatalf("persistChatLog: %v", err)
	}

	ioRow, err := repos().ChatIO.GetByLogID(context.Background(), in.LogID)
	if err != nil {
		t.Fatalf("load chat io: %v", err)
	}
	total := 0
	for _, c := range ioRow.OfStringArray {
		total += len(c)
	}
	limit := getResponseLogMaxBytes(context.Background())
	if total > limit {
		t.Fatalf("ChatIO 累积 %d 字节 > 上限 %d（内存与落库双头都要受预算约束）", total, limit)
	}
}
