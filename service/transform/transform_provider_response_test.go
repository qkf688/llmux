package transform

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/qkf688/llmux/consts"
)

func TestTransformProviderResponse_RewritesContentLengthAndEncodingHeaders(t *testing.T) {
	openaiBody := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"created":123,
		"model":"gpt-4",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
	}`)

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(openaiBody)),
		ContentLength: int64(len(openaiBody)),
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", "9999")
	resp.Header.Set("Content-Encoding", "gzip")
	resp.Header.Set("Transfer-Encoding", "chunked")

	converted, err := TransformProviderResponse(resp, "openai", "openai-res", nil)
	if err != nil {
		t.Fatalf("TransformProviderResponse returned error: %v", err)
	}

	b, err := io.ReadAll(converted.Body)
	if err != nil {
		t.Fatalf("read converted body: %v", err)
	}

	if got := converted.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("expected Content-Encoding to be removed, got %q", got)
	}
	if got := converted.Header.Get("Transfer-Encoding"); got != "" {
		t.Fatalf("expected Transfer-Encoding to be removed, got %q", got)
	}

	if got := converted.Header.Get("Content-Length"); got == "" {
		t.Fatal("expected Content-Length to be set")
	}
	if converted.ContentLength != int64(len(b)) {
		t.Fatalf("expected ContentLength=%d, got %d", len(b), converted.ContentLength)
	}

	hdrLen, err := strconv.Atoi(converted.Header.Get("Content-Length"))
	if err != nil {
		t.Fatalf("invalid Content-Length %q: %v", converted.Header.Get("Content-Length"), err)
	}
	if hdrLen != len(b) {
		t.Fatalf("expected Content-Length header=%d, got %d", len(b), hdrLen)
	}
}

func TestTransformProviderResponse_OpenAIToAnthropic_MapsMultimodalImage(t *testing.T) {
	openaiBody := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"created":123,
		"model":"gpt-4",
		"choices":[{"index":0,"message":{"role":"assistant","content":[
			{"type":"text","text":"hi"},
			{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}}
		]},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
	}`)

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(openaiBody)),
		ContentLength: int64(len(openaiBody)),
	}
	resp.Header.Set("Content-Type", "application/json")

	converted, err := TransformProviderResponse(resp, "openai", "anthropic", nil)
	if err != nil {
		t.Fatalf("TransformProviderResponse returned error: %v", err)
	}

	b, err := io.ReadAll(converted.Body)
	if err != nil {
		t.Fatalf("read converted body: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal converted body: %v", err)
	}

	content, ok := out["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected content blocks, got %#v", out["content"])
	}

	foundText := false
	foundImage := false
	for _, item := range content {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		switch itemMap["type"] {
		case "text":
			if itemMap["text"] == "hi" {
				foundText = true
			}
		case "image":
			source, ok := itemMap["source"].(map[string]interface{})
			if ok && source["type"] == "base64" && source["media_type"] == "image/png" && source["data"] == "AAA" {
				foundImage = true
			}
		}
	}
	if !foundText {
		t.Fatal("expected text block 'hi' in Anthropic content")
	}
	if !foundImage {
		t.Fatal("expected image base64 block in Anthropic content")
	}
}

func TestTransformProviderResponse_SameTypePassthrough(t *testing.T) {
	body := []byte(`{"id":"chatcmpl_1"}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	converted, err := TransformProviderResponse(resp, "openai", "openai", nil)
	if err != nil {
		t.Fatalf("TransformProviderResponse returned error: %v", err)
	}
	if converted != resp {
		t.Fatal("expected same response instance for same provider and client type")
	}
}

// 未注册的 wire format 必须显式报错，**不许**静默回退到 OpenAI 形状。
//
// 本用例替代了原 TestTransformProviderResponse_UnknownTypesFallBackToOpenAI：那条锁的是
// getAdapterOrDefault 的兜底行为，而 adapter.go 已刻意去掉该兜底——回退会把「provider 漏声明
// WireFormat」「新协议漏注册适配器」这类漏配静默变成「按 OpenAI 形状解析上游响应」，
// 现象要飘到客户端解析失败才暴露，离根因很远。
func TestTransformProviderResponse_UnregisteredFormatReturnsError(t *testing.T) {
	openaiBody := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"created":123,
		"model":"gpt-4",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
	}`)

	cases := []struct {
		name           string
		upstreamFormat consts.WireFormat
		clientFormat   consts.WireFormat
	}{
		{
			name:           "unregistered upstream format",
			upstreamFormat: "unregistered-format",
			clientFormat:   consts.FormatOpenAIResponses,
		},
		{
			name:           "unregistered client format",
			upstreamFormat: consts.FormatOpenAIChat,
			clientFormat:   "unregistered-format",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			converted, err := TransformProviderResponse(newJSONResponse(openaiBody), tc.upstreamFormat, tc.clientFormat, nil)
			if err == nil {
				t.Fatalf("未注册形状必须报错，实际转换成功: %s", mustReadResponseBody(t, converted))
			}
		})
	}
}

func newJSONResponse(body []byte) *http.Response {
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func mustReadResponseBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return b
}
