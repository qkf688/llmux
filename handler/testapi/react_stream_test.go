package testapi

import (
	"errors"
	"iter"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atopos31/nsxno/react"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2"
)

// Stage 15 停流契约：ReAct 流循环中任一帧失败（此处为帧数据序列化失败）后，
// 失败帧自身与后续帧都不得再回写 SSE——「error 判定已产生、流却还在输出」的
// 混合事件序列会误导排障。react.ContentResponse 的字段是固定结构体，json.Marshal
// 对它们现实不可失败，无法经输入构造失败场景，故经 frameMarshaler 注入缝驱动。
func TestStreamOpenAIReactEvents_StopsAfterFrameMarshalFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test/react/1", nil)

	marshalErr := errors.New("injected marshal failure")
	failingMarshal := func(v any) (string, error) {
		return "", marshalErr
	}

	seq := func(yield func(*react.ContentResponse, error) bool) {
		// 帧 1：正常 message 帧（失败前产出，应回写）。
		if !yield(&react.ContentResponse{
			Cate: "message",
			Chunk: openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{
				{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "hello"}},
			}},
		}, nil) {
			return
		}
		// 帧 2：toolcall 帧，序列化必失败 → 停流点。
		if !yield(&react.ContentResponse{
			Cate: "toolcall",
			ToolCall: &openai.ChatCompletionChunkChoiceDeltaToolCall{
				Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
					Arguments: `{"location":"北京"}`,
					Name:      "get_weather",
				},
			},
		}, nil) {
			return
		}
		// 帧 3：停流契约生效时不得出现的后续帧。
		yield(&react.ContentResponse{
			Cate: "message",
			Chunk: openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{
				{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "AFTER-FAILURE"}},
			}},
		}, nil)
	}

	cities, final, err := streamOpenAIReactEvents(c, iter.Seq2[*react.ContentResponse, error](seq), failingMarshal)

	if !errors.Is(err, marshalErr) {
		t.Fatalf("err = %v, want 注入的序列化错误", err)
	}
	body := w.Body.String()
	if strings.Contains(body, "AFTER-FAILURE") {
		t.Fatalf("停流失败：失败后仍回写了后续帧；body=%s", body)
	}
	if strings.Contains(body, "toolcall") {
		t.Fatalf("失败帧自身不得回写；body=%s", body)
	}
	if !strings.Contains(body, "hello") {
		t.Fatalf("失败前的正常帧应已回写；body=%s", body)
	}
	// 城市收集在序列化成功之后，失败帧不得计入（防错误数据进场景判定）。
	if len(cities) != 0 {
		t.Fatalf("cities = %v, want 空（失败帧未完成处理不得计入）", cities)
	}
	if final != "hello" {
		t.Fatalf("final = %q, want hello（失败前的正常产出应保留）", final)
	}
}

// 流迭代错误与序列化失败同契约：置错即停，错误之后的帧不得回写。
func TestStreamOpenAIReactEvents_StopsAfterStreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test/react/1", nil)

	streamErr := errors.New("injected stream error")
	seq := func(yield func(*react.ContentResponse, error) bool) {
		if !yield(&react.ContentResponse{
			Cate: "message",
			Chunk: openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{
				{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "first"}},
			}},
		}, nil) {
			return
		}
		if !yield(nil, streamErr) {
			return
		}
		yield(&react.ContentResponse{
			Cate: "message",
			Chunk: openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{
				{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "AFTER-ERROR"}},
			}},
		}, nil)
	}

	_, _, err := streamOpenAIReactEvents(c, iter.Seq2[*react.ContentResponse, error](seq), marshalFrameJSON)

	if !errors.Is(err, streamErr) {
		t.Fatalf("err = %v, want 注入的流错误", err)
	}
	if body := w.Body.String(); strings.Contains(body, "AFTER-ERROR") {
		t.Fatalf("流错误后仍回写了后续帧；body=%s", body)
	}
}
