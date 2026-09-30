package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/gjson"
)

// incidentRawBody 是 2026-09-30 事故（日志 11517）的请求形状：客户端（DeepSeek Harness）
// 在会话中途新增工具后，把 tool_addition 块追加进一条 messages 内的 system 消息。
// 客户端风格是 anthropic、命中的端点协议也是 anthropic → 走直通，私有块原样出站，
// DashScope 的 Anthropic 兼容端点判 400 InvalidParameter "Request body format invalid"。
const incidentRawBody = `{"model":"deepseek-v4.1-flash","max_tokens":256000,"stream":true,` +
	`"thinking":{"type":"enabled"},"output_config":{"effort":"high"},"system":"sys",` +
	`"tools":[{"name":"read_page","input_schema":{"type":"object"}}],` +
	`"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]},` +
	`{"role":"system","content":[{"type":"tool_addition","tool":{"type":"tool_reference","name":"read_page"}}]}]}`

// 直通（anthropic × anthropic）必须把 DeepSeek Messages 私有扩展块收敛成文本：
// 私有块原样出站会被严格上游判 400，且该消息留在历史里让之后每条请求都失败。
func TestBuildRequestBodyForProvider_AnthropicPassthrough_NormalizesToolChangeBlocks(t *testing.T) {
	out, skip, err := buildRequestBodyForProvider(context.Background(), ProviderRequestCaps{
		Style:            string(consts.StyleAnthropic),
		EndpointProtocol: consts.ProtocolAnthropic,
		Raw:              []byte(incidentRawBody),
		SupportsThinking: true,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if skip {
		t.Fatalf("skip = true, want false（同形状必须直通）")
	}

	body := string(out)
	for _, absent := range []string{"tool_addition", "tool_reference"} {
		if strings.Contains(body, absent) {
			t.Errorf("出站 body 仍含私有扩展 %q（严格上游会判 400）:\n%s", absent, body)
		}
	}
	if !strings.Contains(body, `"text":"Tool added: read_page"`) {
		t.Errorf("出站 body 缺工具变更文本（语义被丢掉）:\n%s", body)
	}

	// 归一化只动扩展块：工具声明、system、tool_result、thinking/output_config 全部保持。
	checks := []struct {
		path string
		want string
	}{
		{"tools.0.name", "read_page"},
		{"system", "sys"},
		{"messages.0.content.0.type", "tool_result"},
		{"messages.0.content.0.tool_use_id", "toolu_1"},
		{"messages.1.role", "system"},
		{"messages.1.content.0.type", "text"},
		{"messages.1.content.0.text", "Tool added: read_page"},
		{"thinking.type", "enabled"},
		{"output_config.effort", "high"},
	}
	for _, c := range checks {
		if got := gjson.GetBytes(out, c.path).String(); got != c.want {
			t.Errorf("%s = %q, want %q:\n%s", c.path, got, c.want, body)
		}
	}
}

// 直通的第一纪律是「无改写需求时字节原样」：canonical Anthropic body 不得因归一化而被重序列化
// （键序、数字精度、空白都不能变）。
func TestBuildRequestBodyForProvider_AnthropicPassthrough_CanonicalBodyByteIdentical(t *testing.T) {
	raw := []byte(`{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}],"output_config":{"effort":"high"}}`)
	out, skip, err := buildRequestBodyForProvider(context.Background(), ProviderRequestCaps{
		Style:            string(consts.StyleAnthropic),
		EndpointProtocol: consts.ProtocolAnthropic,
		Raw:              raw,
		SupportsThinking: true,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if skip {
		t.Fatalf("skip = true, want false")
	}
	if string(out) != string(raw) {
		t.Fatalf("无扩展块时 body 被改写:\n got=%s\nwant=%s", out, raw)
	}
}

// 转换线（anthropic 入站 × openai 端点）同样不得让私有块漏进上游，且必须把工具变更语义
// 渲染成文本交给统一模型（此前该块落在 default 分支被静默丢弃）。
func TestBuildRequestBodyForProvider_AnthropicTransform_KeepsToolChangeNotice(t *testing.T) {
	raw := []byte(`{"model":"m","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"tool_addition","tool":{"type":"tool_reference","name":"read_page"}}]}]}`)
	out, skip, err := buildRequestBodyForProvider(context.Background(), ProviderRequestCaps{
		Style:            string(consts.StyleAnthropic),
		EndpointProtocol: consts.ProtocolOpenAI,
		Raw:              raw,
		SupportsThinking: true,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if skip {
		t.Fatalf("skip = true, want false（格式转换开启）")
	}

	body := string(out)
	if strings.Contains(body, "tool_addition") || strings.Contains(body, "tool_reference") {
		t.Errorf("转换产物仍含私有扩展块:\n%s", body)
	}
	if !strings.Contains(body, "Tool added: read_page") {
		t.Errorf("转换产物丢了工具变更语义（应渲染成文本）:\n%s", body)
	}
}
