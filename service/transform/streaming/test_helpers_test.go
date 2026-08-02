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
