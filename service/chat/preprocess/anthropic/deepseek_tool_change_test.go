package anthropic

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// incidentBody 是 2026-09-30 事故（llmux 日志 11517）的请求形状：客户端在会话中途新增工具，
// 把 tool_addition 块追加进一条 messages 内的 system 消息。严格 Anthropic 上游对 content 的
// 块 type 做白名单校验，原样直通必 400，且该消息留在历史里使之后每条请求都失败。
const incidentBody = `{"model":"m","max_tokens":256000,"stream":true,"system":"sys",` +
	`"tools":[{"name":"read_page","input_schema":{"type":"object"}}],` +
	`"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]},` +
	`{"role":"system","content":[` +
	`{"type":"tool_addition","tool":{"type":"tool_reference","name":"read_page"}},` +
	`{"type":"tool_removal","tool":{"type":"tool_reference","name":"x_search"}}]}]}`

func TestNormalizeDeepSeekToolChangeBlocks(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantChanged  bool
		wantEqual    bool // 期望字节完全原样（未改写）
		wantContain  []string
		wantAbsent   []string
		wantMessages int // >0 时断言 messages 数组长度
	}{
		{
			name:        "事故形状：工具变更块降级为文本，其余块与工具声明保持",
			body:        incidentBody,
			wantChanged: true,
			wantContain: []string{
				`"text":"Tool added: read_page"`,
				`"text":"Tool removed: x_search"`,
				`"tool_use_id":"toolu_1"`, // tool_result 块原样保留
				`"system":"sys"`,
				`"read_page","input_schema"`, // tools 数组不受影响
			},
			wantAbsent:   []string{"tool_addition", "tool_removal", "tool_reference"},
			wantMessages: 2,
		},
		{
			name:      "无扩展块：字节原样（含空白与键序，不重序列化）",
			body:      `{"messages": [{"role":"user","content":[{"type":"text","text":"hi"}]}]}`,
			wantEqual: true,
		},
		{
			name:      "content 为字符串：不受影响",
			body:      `{"messages":[{"role":"user","content":"hi"}]}`,
			wantEqual: true,
		},
		{
			name:      "无 messages 数组：原样返回",
			body:      `{"model":"m","tools":[]}`,
			wantEqual: true,
		},
		{
			name:      "非法 JSON：原样返回且不报错",
			body:      `{"messages":[`,
			wantEqual: true,
		},
		{
			name:         "无工具名：块被丢弃；消息删空后整条移除（空 content 数组同样会被上游判 400）",
			body:         `{"messages":[{"role":"user","content":"hi"},{"role":"system","content":[{"type":"tool_addition"}]}]}`,
			wantChanged:  true,
			wantAbsent:   []string{"tool_addition", `"role":"system"`},
			wantMessages: 1,
		},
		{
			name:         "只认线形态 tool.name：平的 toolName 视为无名块（丢块、留痕）",
			body:         `{"messages":[{"role":"user","content":"hi"},{"role":"system","content":[{"type":"tool_addition","toolName":"read_page"}]}]}`,
			wantChanged:  true,
			wantAbsent:   []string{"tool_addition", "Tool added"},
			wantMessages: 1,
		},
		{
			name:        "混排：其它块保留且扩展块就位替换（顺序不变）",
			body:        `{"messages":[{"role":"user","content":[{"type":"text","text":"before"},{"type":"tool_addition","tool":{"type":"tool_reference","name":"x"}},{"type":"text","text":"after"}]}]}`,
			wantChanged: true,
			wantContain: []string{`"before"`, `"text":"Tool added: x"`, `"after"`},
			wantAbsent:  []string{"tool_addition"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed, err := NormalizeDeepSeekToolChangeBlocks([]byte(tt.body))
			if err != nil {
				t.Fatalf("err = %v, want nil（best-effort 不阻断）", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v（body=%s）", changed, tt.wantChanged, got)
			}
			if tt.wantEqual {
				if string(got) != tt.body {
					t.Fatalf("body 被改写了，want 原样:\n got=%s\nwant=%s", got, tt.body)
				}
				return
			}
			s := string(got)
			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("body 缺 %q:\n%s", want, s)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(s, absent) {
					t.Errorf("body 仍含 %q（私有块不得留给上游）:\n%s", absent, s)
				}
			}
			if tt.wantMessages > 0 {
				if n := len(gjson.GetBytes(got, "messages").Array()); n != tt.wantMessages {
					t.Errorf("messages 长度 = %d, want %d:\n%s", n, tt.wantMessages, s)
				}
			}
		})
	}
}

// 归一化必须幂等：重试/重放同一 body 时第二次不得再改（否则说明改写产出又触发了规则）。
func TestNormalizeDeepSeekToolChangeBlocks_Idempotent(t *testing.T) {
	once, changed, err := NormalizeDeepSeekToolChangeBlocks([]byte(incidentBody))
	if err != nil || !changed {
		t.Fatalf("第一次: changed=%v err=%v, want true/nil", changed, err)
	}
	twice, changedAgain, err := NormalizeDeepSeekToolChangeBlocks(once)
	if err != nil {
		t.Fatalf("第二次 err = %v", err)
	}
	if changedAgain {
		t.Fatalf("第二次仍然报告改写:\n%s", twice)
	}
	if string(twice) != string(once) {
		t.Fatalf("非幂等:\n first=%s\nsecond=%s", once, twice)
	}
}
