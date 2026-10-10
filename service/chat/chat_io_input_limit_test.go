package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// Stage 26 回归：ChatIO.Input（请求侧原文）纳入体积上限——与回复侧同一把刀
// （common.TruncateHeadTailUTF8 头尾保留 + 统一标记），预算复用
// raw_log_field_max_kib（请求侧是采样排障语义，不开新设置项，用户拍板）。
// 此前 createChatIO 直写 input 原文，开启 IO 日志 + 大请求体（入站硬顶 20 MiB）
// 时单行 ChatIO 可达 20 MiB。

// rawFieldCapSettingsReader 测试用 settings.Reader stub：只覆写单字段采集上限键。
type rawFieldCapSettingsReader struct {
	MaxKiB int
}

func (r rawFieldCapSettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (r rawFieldCapSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	if key == models.SettingKeyRawLogFieldMaxKiB {
		return r.MaxKiB
	}
	return def
}

func (r rawFieldCapSettingsReader) String(_ context.Context, _ string, def string) string {
	return def
}

func (r rawFieldCapSettingsReader) BoolE(ctx context.Context, key string, def bool) (bool, error) {
	return r.Bool(ctx, key, def), nil
}

func (r rawFieldCapSettingsReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	return r.Int(ctx, key, def, min), nil
}

var _ settings.Reader = rawFieldCapSettingsReader{}

func setRawLogFieldMaxKiB(t *testing.T, kib int) {
	t.Helper()
	installChatTestSettings(t, rawFieldCapSettingsReader{MaxKiB: kib})
}

func TestCreateChatIO_InputCappedHeadTail(t *testing.T) {
	initChatRecordTestDB(t)
	setRawLogFieldMaxKiB(t, 16) // 16 KiB

	payload := strings.Repeat("字", (64<<10)/3) // 64 KiB 中文，截断处多在多字节中间
	logRow := testutil.NewRow(t, models.DB, models.ChatLog{Status: "success"})

	limit := getRawLogFieldMaxBytes(context.Background())
	if err := createChatIO(context.Background(), logRow.ID, []byte(payload), nil); err != nil {
		t.Fatalf("createChatIO: %v", err)
	}
	ioRow, err := repos().ChatIO.GetByLogID(context.Background(), logRow.ID)
	if err != nil {
		t.Fatalf("load chat io: %v", err)
	}

	got := ioRow.Input
	// 标记字节计入预算：总长 ≤ 上限（与回复侧 truncateResponseBody 同一口径）。
	if len(got) > limit {
		t.Fatalf("ChatIO.Input 落库 %d 字节 > 上限 %d（含标记）", len(got), limit)
	}
	if !strings.Contains(got, "内容过长已截断") {
		t.Fatalf("超限必须带统一头尾截断标记")
	}
	// 头段是原文前缀、尾段是原文后缀（rune 边界内各取 12 字节核对）。
	if !strings.HasPrefix(payload, got[:12]) {
		t.Fatalf("头段不是原文前缀")
	}
	if !strings.HasSuffix(payload, got[len(got)-12:]) {
		t.Fatalf("尾段不是原文后缀")
	}
}

func TestCreateChatIO_InputWithinLimitUnchanged(t *testing.T) {
	initChatRecordTestDB(t)
	setRawLogFieldMaxKiB(t, 16)

	payload := strings.Repeat("a", 4<<10) // 4 KiB，远低于 16 KiB 上限
	logRow := testutil.NewRow(t, models.DB, models.ChatLog{Status: "success"})

	if err := createChatIO(context.Background(), logRow.ID, []byte(payload), nil); err != nil {
		t.Fatalf("createChatIO: %v", err)
	}
	ioRow, err := repos().ChatIO.GetByLogID(context.Background(), logRow.ID)
	if err != nil {
		t.Fatalf("load chat io: %v", err)
	}
	if ioRow.Input != payload {
		t.Fatalf("上限内 Input 原样保留（got %d bytes, want %d）", len(ioRow.Input), len(payload))
	}
}
