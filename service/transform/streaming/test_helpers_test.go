package streaming

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type sseEvent struct {
	name string
	data map[string]interface{}
}

func (e sseEvent) typ() string {
	if t, _ := e.data["type"].(string); t != "" {
		return t
	}
	return e.name
}

func (e sseEvent) index(t *testing.T) int {
	t.Helper()
	return e.intField(t, "index")
}

func (e sseEvent) outputIndex(t *testing.T) int {
	t.Helper()
	return e.intField(t, "output_index")
}

func (e sseEvent) intField(t *testing.T, key string) int {
	t.Helper()
	v, ok := e.data[key].(float64)
	if !ok {
		t.Fatalf("事件 %q 缺少数值字段 %q: %v", e.typ(), key, e.data)
	}
	return int(v)
}

func runRealtimeTransform(t *testing.T, in, from, to string) []sseEvent {
	t.Helper()

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(in)),
	}

	out, err := TransformResponseRealtime(resp, from, to, nil)
	if err != nil {
		t.Fatalf("TransformResponseRealtime(%s->%s) 失败: %v", from, to, err)
	}
	defer out.Body.Close()

	var events []sseEvent
	var eventName string
	scanner := bufio.NewScanner(out.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			eventName = ""
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			t.Fatalf("输出流中出现无法解析的 JSON: %s (%v)", data, err)
		}
		events = append(events, sseEvent{name: eventName, data: payload})
		eventName = ""
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("读取输出流失败: %v", err)
	}

	if len(events) == 0 {
		t.Fatalf("转换 %s->%s 未产出任何事件", from, to)
	}
	return events
}

// findEvent 返回首个匹配 typ 的事件，找不到即 Fatal。
func findEvent(t *testing.T, events []sseEvent, typ string) sseEvent {
	t.Helper()
	for _, ev := range events {
		if ev.typ() == typ {
			return ev
		}
	}
	t.Fatalf("输出流中未出现事件 %q，实际事件序列: %v", typ, eventTypes(events))
	return sseEvent{}
}

// countEvents 统计匹配 typ 的事件数量。
func countEvents(events []sseEvent, typ string) int {
	n := 0
	for _, ev := range events {
		if ev.typ() == typ {
			n++
		}
	}
	return n
}

func eventTypes(events []sseEvent) []string {
	types := make([]string, 0, len(events))
	for _, ev := range events {
		types = append(types, ev.typ())
	}
	return types
}

// nestedMap 沿 keys 逐层取嵌套对象字段，任一层缺失或非对象即 Fatal。
func nestedMap(t *testing.T, m map[string]interface{}, keys ...string) map[string]interface{} {
	t.Helper()
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			t.Fatalf("字段路径 %v 在 %q 处缺失或非对象: %v", keys, k, cur)
		}
		cur = next
	}
	return cur
}

// assertNumber 断言裸 map 上的数值字段等于 want。
// 与 sseEvent.intField 的区别：后者是事件方法且只取值不比较。
func assertNumber(t *testing.T, m map[string]interface{}, key string, want int) {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("缺少数值字段 %q: %v", key, m)
	}
	if int(v) != want {
		t.Fatalf("字段 %q = %d，期望 %d", key, int(v), want)
	}
}

// newTestRealtimeState 构造一个把输出丢弃的 state，供需要直接驱动
// handler 并检查 state 内部字段的白盒测试使用。
// 返回的 cleanup 必须调用，否则 drain goroutine 泄漏。
func newTestRealtimeState(t *testing.T, providerType, clientType string) (*realtimeStreamState, func()) {
	t.Helper()

	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, pr)
	}()

	state := &realtimeStreamState{
		writer:                     pw,
		providerType:               providerType,
		clientType:                 clientType,
		anthropicActiveBlockIndex:  -1,
		anthropicActiveOutputIndex: -1,
	}

	return state, func() {
		_ = pw.Close()
		<-done
	}
}
