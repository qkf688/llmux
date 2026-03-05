package transform

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTransformStreamResponseRealtime_ResponsesToOpenAI_StableMeta(t *testing.T) {
	responsesStream := `event: response.created
data: {"type":"response.created","response":{"id":"resp_123","object":"response","model":"gpt-4","created_at":1234567890,"status":"in_progress","output":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","response_id":"resp_123","delta":"Hello"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","response_id":"resp_123","delta":" world"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_123","object":"response","model":"gpt-4","created_at":1234567890,"status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}
`

	response := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(responsesStream)),
	}

	result, err := transformStreamResponseRealtime(response, "openai-res", "openai")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}
	defer result.Body.Close()

	scanner := bufio.NewScanner(result.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var ids []string
	var models []string
	var createds []int64

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("解析输出事件失败: %v, data=%s", err, data)
		}

		id, _ := ev["id"].(string)
		model, _ := ev["model"].(string)
		createdFloat, _ := ev["created"].(float64)

		ids = append(ids, id)
		models = append(models, model)
		createds = append(createds, int64(createdFloat))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	if len(ids) == 0 {
		t.Fatal("没有解析到任何输出事件")
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] != ids[0] {
			t.Fatalf("输出 id 不稳定: %q vs %q", ids[0], ids[i])
		}
		if models[i] != models[0] {
			t.Fatalf("输出 model 不稳定: %q vs %q", models[0], models[i])
		}
		if createds[i] != createds[0] {
			t.Fatalf("输出 created 不稳定: %d vs %d", createds[0], createds[i])
		}
	}

	if ids[0] != "chatcmpl-resp_123" {
		t.Fatalf("输出 id 应为 chatcmpl-resp_123，实际: %q", ids[0])
	}
	if models[0] != "gpt-4" {
		t.Fatalf("输出 model 应为 gpt-4，实际: %q", models[0])
	}
	if createds[0] != 1234567890 {
		t.Fatalf("输出 created 应为 1234567890，实际: %d", createds[0])
	}
}

func TestTransformStreamResponseRealtime_AnthropicToOpenAI_StableMeta(t *testing.T) {
	anthropicStream := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-5-sonnet","created_at":111,"usage":{"input_tokens":0,"output_tokens":0}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":1,"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}
`

	response := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(anthropicStream)),
	}

	result, err := transformStreamResponseRealtime(response, "anthropic", "openai")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}
	defer result.Body.Close()

	scanner := bufio.NewScanner(result.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var ids []string
	var models []string
	var createds []int64

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("解析输出事件失败: %v, data=%s", err, data)
		}

		id, _ := ev["id"].(string)
		model, _ := ev["model"].(string)
		createdFloat, _ := ev["created"].(float64)

		ids = append(ids, id)
		models = append(models, model)
		createds = append(createds, int64(createdFloat))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	if len(ids) == 0 {
		t.Fatal("没有解析到任何输出事件")
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] != ids[0] {
			t.Fatalf("输出 id 不稳定: %q vs %q", ids[0], ids[i])
		}
		if models[i] != models[0] {
			t.Fatalf("输出 model 不稳定: %q vs %q", models[0], models[i])
		}
		if createds[i] != createds[0] {
			t.Fatalf("输出 created 不稳定: %d vs %d", createds[0], createds[i])
		}
	}

	if ids[0] != "chatcmpl-msg_123" {
		t.Fatalf("输出 id 应为 chatcmpl-msg_123，实际: %q", ids[0])
	}
	if models[0] != "claude-3-5-sonnet" {
		t.Fatalf("输出 model 应为 claude-3-5-sonnet，实际: %q", models[0])
	}
	if createds[0] != 111 {
		t.Fatalf("输出 created 应为 111，实际: %d", createds[0])
	}
}

