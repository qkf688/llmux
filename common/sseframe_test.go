package common

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// collectFrames 跑一遍 ScanSSEFrames 并返回（帧, stats, err），onFrame 永远放行。
func collectFrames(t *testing.T, input string, cfg SSEScanConfig) ([]SSEFrame, SSEScanStats, error) {
	t.Helper()
	var got []SSEFrame
	stats, err := ScanSSEFrames(strings.NewReader(input), cfg, func(frame SSEFrame, _ SSEScanStats) error {
		got = append(got, frame)
		return nil
	})
	return got, stats, err
}

// TestScanSSEFrames_Golden 钉死「什么是一条 SSE 事件」的唯一判据。
// 期望帧全部来自三份历史实现（credprobe/网关/chat 日志）的并集语义：
// 空行交付、多行 data 以 \n 合并、event: 记名、: 注释忽略、EOF 补交付、
// data: 载荷只去前导一个空格（SSE 规范式，不去行尾空白、不TrimTab）。
func TestScanSSEFrames_Golden(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []SSEFrame
	}{
		{
			name:  "openai chat 无 event 行",
			input: "data: {\"a\":1}\n\ndata: [DONE]\n\n",
			want:  []SSEFrame{{Data: `{"a":1}`}, {Data: "[DONE]"}},
		},
		{
			name:  "event 记名且跨帧重置",
			input: "event: message_start\ndata: {\"type\":\"message_start\"}\n\ndata: bare\n\n",
			want:  []SSEFrame{{Event: "message_start", Data: `{"type":"message_start"}`}, {Data: "bare"}},
		},
		{
			name:  "多行 data 以换行合并",
			input: "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\ndata:  \"index\":0}\n\n",
			want:  []SSEFrame{{Event: "content_block_delta", Data: "{\"type\":\"content_block_delta\",\n \"index\":0}"}},
		},
		{
			name:  "注释行不成帧且不断帧",
			input: ": keep-alive\n\ndata: x\n\n",
			want:  []SSEFrame{{Data: "x"}},
		},
		{
			name:  "id retry 与裸行被忽略但仍可成帧",
			input: "id: 5\nretry: 100\nnonsense\ndata: x\n\n",
			want:  []SSEFrame{{Data: "x"}},
		},
		{
			name:  "仅有 event 无 data 不交付",
			input: "event: ping\n\n",
			want:  nil,
		},
		{
			name:  "CRLF 行尾",
			input: "event: e\r\ndata: x\r\n\r\n",
			want:  []SSEFrame{{Event: "e", Data: "x"}},
		},
		{
			name:  "EOF 无末空行仍交付末帧",
			input: "event: message_stop\ndata: {}",
			want:  []SSEFrame{{Event: "message_stop", Data: "{}"}},
		},
		{
			name:  "空 data 行不入载荷",
			input: "data: a\ndata:\ndata: \ndata: b\n\n",
			want:  []SSEFrame{{Data: "a\nb"}},
		},
		{
			name: "规范式 trim 只去一个前导空格",
			// AC-4：载荷保留第二个前导空格与全部行尾空白（旧网关/chat 的
			// TrimSpace 语义即在此被有意改变）。
			input: "data:   x   \n\ndata:\tpayload\n\n",
			want:  []SSEFrame{{Data: "  x   "}, {Data: "\tpayload"}},
		},
		{
			name:  "事件名两端空白修剪",
			input: "event:   ping  \ndata: x\n\n",
			want:  []SSEFrame{{Event: "ping", Data: "x"}},
		},
		{
			name:  "完全空输入零帧",
			input: "",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := collectFrames(t, tc.input, SSEScanConfig{})
			if err != nil {
				t.Fatalf("ScanSSEFrames 出错: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("帧数 = %d, want %d (%+v)", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("frames[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestScanSSEFrames_StatsAndEarlyStop(t *testing.T) {
	input := "data: a\n\ndata: b\n\ndata: c\n\n"

	t.Run("stats 计行数与帧数", func(t *testing.T) {
		got, stats, err := collectFrames(t, input, SSEScanConfig{})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(got) != 3 || stats.Frames != 3 {
			t.Fatalf("frames = %d/%d, want 3", len(got), stats.Frames)
		}
		// 3 个 data 行 + 3 个空行
		if stats.Lines != 6 {
			t.Fatalf("stats.Lines = %d, want 6", stats.Lines)
		}
	})

	t.Run("回调返错立即停止且错误原样返回", func(t *testing.T) {
		sentinel := errors.New("boom")
		var seen []SSEFrame
		stats, err := ScanSSEFrames(strings.NewReader(input), SSEScanConfig{}, func(frame SSEFrame, _ SSEScanStats) error {
			seen = append(seen, frame)
			if frame.Data == "b" {
				return sentinel
			}
			return nil
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want 原样哨兵", err)
		}
		if len(seen) != 2 || stats.Frames != 2 {
			t.Fatalf("停止后仍收了 %d 帧 (stats.Frames=%d), want 2", len(seen), stats.Frames)
		}
	})

	t.Run("回调可见当前统计快照", func(t *testing.T) {
		var lastLines int
		_, err := ScanSSEFrames(strings.NewReader(input), SSEScanConfig{}, func(_ SSEFrame, stats SSEScanStats) error {
			lastLines = stats.Lines
			return nil
		})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if lastLines == 0 || lastLines > 6 {
			t.Fatalf("回调内 stats.Lines = %d, want (0,6] 区间内的中途值", lastLines)
		}
	})
}

func TestScanSSEFrames_Budgets(t *testing.T) {
	// 4 帧、每帧 "data: x\n\n"（9 字节）。
	input := strings.Repeat("data: x\n\n", 4)

	t.Run("字节预算按消费字节计数", func(t *testing.T) {
		got, stats, err := collectFrames(t, input, SSEScanConfig{MaxTotalBytes: 20})
		var budgetErr *SSEBudgetError
		if !errors.As(err, &budgetErr) {
			t.Fatalf("err = %v, want *SSEBudgetError", err)
		}
		if budgetErr.Kind != SSEBudgetBytes || budgetErr.Limit != 20 {
			t.Fatalf("budgetErr = %+v, want bytes/20", *budgetErr)
		}
		if budgetErr.Error() != fmt.Sprintf("字节数超过 %d 预算", 20) {
			t.Fatalf("Error() = %q, want 旧探针文案口径", budgetErr.Error())
		}
		// 预算错误路径不得再交付 EOF 末帧：已交付帧数必须与 stats 一致
		if len(got) != stats.Frames {
			t.Fatalf("got %d 帧但 stats.Frames = %d", len(got), stats.Frames)
		}
	})

	t.Run("帧数预算封住小帧洪水", func(t *testing.T) {
		got, _, err := collectFrames(t, input, SSEScanConfig{MaxFrames: 2})
		var budgetErr *SSEBudgetError
		if !errors.As(err, &budgetErr) {
			t.Fatalf("err = %v, want *SSEBudgetError", err)
		}
		if budgetErr.Kind != SSEBudgetFrames || budgetErr.Limit != 2 {
			t.Fatalf("budgetErr = %+v, want frames/2", *budgetErr)
		}
		if len(got) > 2 {
			t.Fatalf("超预算后仍交付 %d 帧, want ≤2", len(got))
		}
	})

	t.Run("行预算走 bufio.ErrTooLong 而非预算错误", func(t *testing.T) {
		long := "data: " + strings.Repeat("z", 200) + "\n\n"
		_, _, err := collectFrames(t, long, SSEScanConfig{MaxLineBytes: 64})
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Fatalf("err = %v, want bufio.ErrTooLong（与历史探针「超长单行归读失败」口径一致）", err)
		}
		var budgetErr *SSEBudgetError
		if errors.As(err, &budgetErr) {
			t.Fatal("超长行不应归为预算中止")
		}
	})

	t.Run("预算为零即不限", func(t *testing.T) {
		got, _, err := collectFrames(t, input, SSEScanConfig{})
		if err != nil || len(got) != 4 {
			t.Fatalf("零值 cfg 不应改变判定: frames=%d err=%v", len(got), err)
		}
	})
}

func TestScanSSEFrames_ReadErrorPropagates(t *testing.T) {
	sentinel := errors.New("reader exploded")
	_, err := ScanSSEFrames(io.MultiReader(
		strings.NewReader("data: a\n\n"),
		errReader{err: sentinel},
	), SSEScanConfig{}, func(SSEFrame, SSEScanStats) error { return nil })
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestIsEventStreamContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"text/event-stream", true},
		{"text/event-stream; charset=utf-8", true},
		{"application/json; text/event-stream", true}, // Contains 语义（判据与转发链路一致，勿改 HasPrefix）
		{"TEXT/EVENT-STREAM", true},
		{"  text/event-stream ", true},
		{"application/json", false},
		{"", false},
		{"text/event", false}, // 残缺媒体类型不算流（探针不得对着 JSON 分帧）
	}
	for _, tc := range tests {
		if got := IsEventStreamContentType(tc.input); got != tc.want {
			t.Errorf("IsEventStreamContentType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
