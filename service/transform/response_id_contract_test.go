package transform

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/transform/shared"
)

// 本文件锁「转换产物的响应 id 必须自述目标协议」这条契约。
//
// 为什么单独立一个契约测试而不靠 golden：golden 只记录「现在输出什么」，把双前缀
// 也一并冻住了（2026-10-24 之前 `chatcmpl-resp_123` / `msg_chatcmpl_123` 都在基线里）。
// 这条断言不看具体值，只看**前缀与目标协议是否一致**，任何协议对、任何裸 id 都适用。

// idBearingFixture 每条上游协议的**成功响应**样本（id 用该协议自己的原生形状）。
var idBearingFixture = map[consts.WireFormat]string{
	consts.FormatOpenAIChat: `{"id":"chatcmpl-UPSTREAM","object":"chat.completion","created":1,"model":"m",
		"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
	consts.FormatOpenAIResponses: `{"id":"resp_UPSTREAM","object":"response","created_at":1,"status":"completed",
		"output":[{"id":"msg_UPSTREAM","type":"message","role":"assistant",
			"content":[{"type":"output_text","text":"ok"}]}],
		"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
	consts.FormatAnthropic: `{"id":"msg_UPSTREAM","type":"message","role":"assistant","model":"m",
		"content":[{"type":"thinking","thinking":"x","signature":"sig"},{"type":"text","text":"ok"}],
		"stop_reason":"end_turn","stop_sequence":null,
		"usage":{"input_tokens":1,"output_tokens":1}}`,
}

// TestTransformProviderResponse_ResponseIDMatchesTargetProtocol 遍历全部 6 个跨协议
// 组合，断言出站 body 的响应级 id 以目标协议的原生前缀开头、且不含任何别的协议前缀。
//
// 反向断言（不含别的协议前缀）是这条契约的核心：只查「以目标前缀开头」会漏掉
// `chatcmpl-msg_...` 这类**叠加**形态——它以正确前缀开头，但把上游协议身份也带了出来。
func TestTransformProviderResponse_ResponseIDMatchesTargetProtocol(t *testing.T) {
	t.Parallel()

	formats := []consts.WireFormat{consts.FormatOpenAIChat, consts.FormatOpenAIResponses, consts.FormatAnthropic}
	for _, upstream := range formats {
		upstream := upstream
		for _, client := range formats {
			if client == upstream {
				continue // 同形状直通，id 原样透传是正确行为
			}
			client := client
			t.Run(string(upstream)+"_to_"+string(client), func(t *testing.T) {
				t.Parallel()

				resp := &http.Response{
					StatusCode: 200,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader([]byte(idBearingFixture[upstream]))),
				}
				outResp, err := TransformProviderResponse(resp, upstream, client, nil)
				if err != nil {
					t.Fatalf("TransformProviderResponse(%s->%s) failed: %v", upstream, client, err)
				}
				defer outResp.Body.Close()
				out, err := io.ReadAll(outResp.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}

				var decoded struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(out, &decoded); err != nil {
					t.Fatalf("decode body: %v (body=%s)", err, out)
				}

				wantPrefix := shared.ResponseIDPrefix(client)
				if wantPrefix == "" {
					t.Fatalf("目标协议 %s 未注册 id 前缀（新增协议漏配 shared.responseIDPrefixes）", client)
				}
				if !bytes.HasPrefix([]byte(decoded.ID), []byte(wantPrefix)) {
					t.Fatalf("%s -> %s 的响应 id = %q, 应以目标协议前缀 %q 开头",
						upstream, client, decoded.ID, wantPrefix)
				}
				// 反向：不得残留任何别的协议前缀（叠加形态的检测点）。
				for _, other := range formats {
					if other == client {
						continue
					}
					token := shared.ResponseIDPrefixToken(other)
					if token == "" {
						continue
					}
					for _, sep := range []string{"_", "-"} {
						if bytes.Contains([]byte(decoded.ID), []byte(token+sep)) {
							t.Fatalf("%s -> %s 的响应 id = %q 残留了 %s 的协议前缀（叠加形态）",
								upstream, client, decoded.ID, other)
						}
					}
				}
			})
		}
	}
}

// Responses 出站的 output item id 前缀固定 `msg_`：客户端把 output 回灌成下一轮 input
// 时按 `msg_` 找 item，给成 `resp_` 形状会找不到。
func TestTransformProviderResponse_ResponsesItemIDPrefix(t *testing.T) {
	t.Parallel()

	for _, upstream := range []consts.WireFormat{consts.FormatOpenAIChat, consts.FormatAnthropic} {
		upstream := upstream
		t.Run(string(upstream), func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(idBearingFixture[upstream]))),
			}
			outResp, err := TransformProviderResponse(resp, upstream, consts.FormatOpenAIResponses, nil)
			if err != nil {
				t.Fatalf("TransformProviderResponse(%s->responses) failed: %v", upstream, err)
			}
			defer outResp.Body.Close()
			out, err := io.ReadAll(outResp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			var decoded struct {
				Output []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
				} `json:"output"`
			}
			if err := json.Unmarshal(out, &decoded); err != nil {
				t.Fatalf("decode body: %v (body=%s)", err, out)
			}
			foundMessage := false
			for _, item := range decoded.Output {
				if item.Type != "message" {
					continue
				}
				foundMessage = true
				if !bytes.HasPrefix([]byte(item.ID), []byte("msg_")) {
					t.Fatalf("%s -> responses 的 message item id = %q, want msg_ 前缀", upstream, item.ID)
				}
				if bytes.Contains([]byte(item.ID), []byte("resp_")) || bytes.Contains([]byte(item.ID), []byte("chatcmpl")) {
					t.Fatalf("%s -> responses 的 message item id = %q 残留了响应级前缀", upstream, item.ID)
				}
			}
			if !foundMessage {
				t.Fatalf("%s -> responses 没有产出 message item: %s", upstream, out)
			}
		})
	}
}
