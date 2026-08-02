package streaming

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestStreamConversion_AnthropicToResponses 测试 Anthropic → OpenAI Responses 流式转换
func TestStreamConversion_AnthropicToResponses(t *testing.T) {
	// 模拟 Anthropic SSE 流
	anthropicStream := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`

	// 创建模拟响应
	response := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(anthropicStream)),
	}

	// 执行转换
	result, err := TransformResponseRealtime(response, "anthropic", "openai-res", nil)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 读取转换后的流
	scanner := bufio.NewScanner(result.Body)
	var events []map[string]interface{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				continue
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				t.Logf("跳过无效 JSON: %s", data)
				continue
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	// 验证事件序列
	if len(events) == 0 {
		t.Fatal("未收到任何事件")
	}

	// 验证第一个事件是 response.created
	if events[0]["type"] != "response.created" {
		t.Errorf("第一个事件应为 response.created，实际: %v", events[0]["type"])
	}

	// 验证包含 output_text.delta 事件
	foundTextDelta := false
	for _, event := range events {
		if event["type"] == "response.output_text.delta" {
			foundTextDelta = true
			if delta, ok := event["delta"].(string); ok {
				if delta != "Hello" && delta != " world" {
					t.Logf("收到文本增量: %s", delta)
				}
			}
		}
	}
	if !foundTextDelta {
		t.Error("未找到 response.output_text.delta 事件")
	}

	// 验证最后一个事件是 response.completed
	lastEvent := events[len(events)-1]
	if lastEvent["type"] != "response.completed" {
		t.Errorf("最后一个事件应为 response.completed，实际: %v", lastEvent["type"])
	}

	// 验证 response.completed 包含 usage 并校验值
	respData, ok := lastEvent["response"].(map[string]interface{})
	if !ok {
		t.Fatal("response.completed 缺少 response 字段")
	}
	usage, ok := respData["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("response.completed 缺少 usage 字段")
	}
	if inputTokens, ok := usage["input_tokens"].(float64); !ok || inputTokens != 10 {
		t.Errorf("input_tokens 应为 10，实际: %v", usage["input_tokens"])
	}
	if outputTokens, ok := usage["output_tokens"].(float64); !ok || outputTokens != 5 {
		t.Errorf("output_tokens 应为 5，实际: %v", usage["output_tokens"])
	}
}

// TestStreamConversion_ResponsesToAnthropic 测试 OpenAI Responses → Anthropic 流式转换
func TestStreamConversion_ResponsesToAnthropic(t *testing.T) {
	// 模拟 OpenAI Responses SSE 流
	responsesStream := `event: response.created
data: {"type":"response.created","response":{"id":"resp_123","object":"realtime.response","status":"in_progress","output":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"output_text","text":""}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":0,"delta":"Hello"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":0,"delta":" world"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_123","status":"completed","output":[{"type":"output_text","text":"Hello world"}],"usage":{"input_tokens":10,"output_tokens":5}}}

`

	response := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(responsesStream)),
	}

	result, err := TransformResponseRealtime(response, "openai-res", "anthropic", nil)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	scanner := bufio.NewScanner(result.Body)
	var events []map[string]interface{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				continue
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("未收到任何事件")
	}

	// 验证事件类型和顺序
	foundMessageStart := false
	foundContentBlockDelta := false
	foundContentBlockStop := false
	foundMessageDelta := false
	foundMessageStop := false

	contentBlockStopIdx := -1
	messageDeltaIdx := -1
	messageStopIdx := -1

	for i, event := range events {
		switch event["type"] {
		case "message_start":
			foundMessageStart = true
		case "content_block_delta":
			foundContentBlockDelta = true
		case "content_block_stop":
			foundContentBlockStop = true
			contentBlockStopIdx = i
		case "message_delta":
			foundMessageDelta = true
			messageDeltaIdx = i
		case "message_stop":
			foundMessageStop = true
			messageStopIdx = i
		}
	}

	if !foundMessageStart {
		t.Error("未找到 message_start 事件")
	}
	if !foundContentBlockDelta {
		t.Error("未找到 content_block_delta 事件")
	}
	if !foundContentBlockStop {
		t.Error("未找到 content_block_stop 事件")
	}
	if !foundMessageDelta {
		t.Error("未找到 message_delta 事件")
	}
	if !foundMessageStop {
		t.Error("未找到 message_stop 事件")
	}

	// 验证事件顺序：content_block_stop -> message_delta -> message_stop
	if contentBlockStopIdx >= 0 && messageDeltaIdx >= 0 && contentBlockStopIdx >= messageDeltaIdx {
		t.Error("content_block_stop 应在 message_delta 之前")
	}
	if messageDeltaIdx >= 0 && messageStopIdx >= 0 && messageDeltaIdx >= messageStopIdx {
		t.Error("message_delta 应在 message_stop 之前")
	}
}

// TestStreamConversion_ToolCalls 测试工具调用的流式转换
func TestStreamConversion_ToolCalls(t *testing.T) {
	// 测试 OpenAI Chat → OpenAI Responses 工具调用转换
	openaiStream := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

	response := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(openaiStream)),
	}

	result, err := TransformResponseRealtime(response, "openai", "openai-res", nil)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	scanner := bufio.NewScanner(result.Body)
	var events []map[string]interface{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				continue
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	// 验证工具调用事件（只统计 function_call 类型）
	foundFunctionCallItemAdded := false
	foundFunctionCallArgs := false
	foundCompleted := false
	var functionCallID, functionCallName string

	for _, event := range events {
		switch event["type"] {
		case "response.output_item.added":
			// 只统计 function_call 类型的 item
			item, ok := event["item"].(map[string]interface{})
			if !ok {
				t.Fatal("response.output_item.added 缺少 item 字段")
			}
			if item["type"] == "function_call" {
				foundFunctionCallItemAdded = true
				// 验证包含工具调用元数据
				id, hasID := item["id"].(string)
				name, hasName := item["name"].(string)
				if !hasID || id == "" {
					t.Error("function_call 缺少 id")
				}
				if !hasName || name == "" {
					t.Error("function_call 缺少 name")
				}
				functionCallID = id
				functionCallName = name
			}
		case "response.function_call_arguments.delta":
			foundFunctionCallArgs = true
		case "response.completed":
			foundCompleted = true
			// 验证 status 为 completed
			resp, ok := event["response"].(map[string]interface{})
			if !ok {
				t.Fatal("response.completed 缺少 response 字段")
			}
			status, ok := resp["status"].(string)
			if !ok || status != "completed" {
				t.Errorf("response.completed 的 status 应为 'completed'，实际: %v", resp["status"])
			}
			// 验证 usage 存在且值正确
			usage, ok := resp["usage"].(map[string]interface{})
			if !ok {
				t.Fatal("response.completed 缺少 usage 字段")
			}
			inputTokens, hasInput := usage["input_tokens"].(float64)
			outputTokens, hasOutput := usage["output_tokens"].(float64)
			totalTokens, hasTotal := usage["total_tokens"].(float64)
			if !hasInput || inputTokens != 10 {
				t.Errorf("input_tokens 应为 10，实际: %v", usage["input_tokens"])
			}
			if !hasOutput || outputTokens != 5 {
				t.Errorf("output_tokens 应为 5，实际: %v", usage["output_tokens"])
			}
			if !hasTotal || totalTokens != 15 {
				t.Errorf("total_tokens 应为 15，实际: %v", usage["total_tokens"])
			}
		}
	}

	if !foundFunctionCallItemAdded {
		t.Error("未找到 function_call 类型的 response.output_item.added 事件")
	}
	if !foundFunctionCallArgs {
		t.Error("未找到 response.function_call_arguments.delta 事件")
	}
	if !foundCompleted {
		t.Error("未找到 response.completed 事件")
	}
	if functionCallID == "" || functionCallName == "" {
		t.Error("function_call 元数据不完整")
	}
}

// TestStreamConversion_NoDoubleTermination 测试不会重复发送终止信号
func TestStreamConversion_NoDoubleTermination(t *testing.T) {
	responsesStream := `event: response.completed
data: {"type":"response.completed","response":{"id":"resp_123","status":"completed","output":[{"type":"output_text","text":"Done"}],"usage":{"input_tokens":10,"output_tokens":5}}}

data: [DONE]

`

	response := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(responsesStream)),
	}

	// 测试 openai-res → anthropic
	result, err := TransformResponseRealtime(response, "openai-res", "anthropic", nil)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	scanner := bufio.NewScanner(result.Body)
	messageStopCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event["type"] == "message_stop" {
				messageStopCount++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	if messageStopCount != 1 {
		t.Errorf("message_stop 事件应发送 1 次，实际: %d 次", messageStopCount)
	}
}

// TestStreamConversion_UsageInCompletedEvent 测试 usage 在完成事件中
func TestStreamConversion_UsageInCompletedEvent(t *testing.T) {
	anthropicStream := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Test"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"output_tokens":5}}

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

	result, err := TransformResponseRealtime(response, "anthropic", "openai-res", nil)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	scanner := bufio.NewScanner(result.Body)
	foundUsageInCompleted := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event["type"] == "response.completed" {
				resp, ok := event["response"].(map[string]interface{})
				if !ok {
					t.Fatal("response.completed 缺少 response 字段")
				}
				usage, ok := resp["usage"].(map[string]interface{})
				if !ok {
					t.Fatal("response.completed 缺少 usage 字段")
				}
				inputTokens, hasInput := usage["input_tokens"].(float64)
				outputTokens, hasOutput := usage["output_tokens"].(float64)
				if hasInput && hasOutput && inputTokens == 10 && outputTokens == 5 {
					foundUsageInCompleted = true
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("扫描流时出错: %v", err)
	}

	if !foundUsageInCompleted {
		t.Error("response.completed 事件中未找到完整的 usage 信息（input_tokens=10, output_tokens=5）")
	}
}

// BenchmarkStreamConversion 性能基准测试
func BenchmarkStreamConversion(b *testing.B) {
	stream := `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := &http.Response{
			StatusCode: 200,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(stream)),
		}

		result, err := TransformResponseRealtime(response, "anthropic", "openai-res", nil)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, result.Body)
		result.Body.Close()
	}
}
