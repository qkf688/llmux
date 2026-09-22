// 载荷「内容有效性」判据（本文件）：网关在把上游响应提交给客户端之前，用它判
// 「200 但内容是错误载荷 / 垃圾」。为什么在 common：与 sseframe.go 的分帧判据同族
// （纯函数、无业务依赖），chat 提交门与后续检测类内容门共用一份错误形状判断，
// 避免各写一套（AGENTS 4.4 DRY）。
//
// 错误形状判据 wire 无关：三家客户端协议（OpenAI Chat / Anthropic / Responses）的
// 错误载荷都带非 null 的顶层 error 键或 type:"error"（流式另有 event: error 行），
// 无需按协议分发。`[DONE]` 是 OpenAI Chat 的合法终止符，免检放行。
package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// sseDoneMarker 是 OpenAI Chat 流式终止符（非 JSON，特判放行）。
const sseDoneMarker = "[DONE]"

// contentErrorPreviewBytes 是错误文本里携带的载荷预览上限（仅进日志，防日志放大）。
const contentErrorPreviewBytes = 200

// JSONContentError 判定一段 JSON 载荷是否可作为「成功响应内容」提交给客户端。
// 返回非 nil = 不可提交（空体 / 非法 JSON / 非对象 / 错误载荷），错误文本仅进日志。
func JSONContentError(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.New("empty body")
	}
	if !json.Valid(trimmed) {
		return fmt.Errorf("not valid JSON: %s", contentErrorPreview(trimmed))
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		// 合法 JSON 但非对象（数组/标量）：聊天响应恒为对象，按不可提交处置。
		return fmt.Errorf("JSON is not an object: %s", contentErrorPreview(trimmed))
	}
	if v, ok := probe["error"]; ok && string(v) != "null" {
		return fmt.Errorf("error payload: %s", contentErrorPreview(trimmed))
	}
	if v, ok := probe["type"]; ok && string(v) == `"error"` {
		return fmt.Errorf("error payload: %s", contentErrorPreview(trimmed))
	}
	return nil
}

// SSEFrameContentError 判定一帧 SSE 载荷是否可作为首个提交内容：event: error 行
// 直接判错误载荷；[DONE] 终止符放行；其余走 JSONContentError。
func SSEFrameContentError(frame SSEFrame) error {
	if frame.Event == "error" {
		return fmt.Errorf("error event: %s", contentErrorPreview([]byte(frame.Data)))
	}
	if strings.TrimSpace(frame.Data) == sseDoneMarker {
		return nil
	}
	return JSONContentError([]byte(frame.Data))
}

func contentErrorPreview(data []byte) string {
	return TruncateUTF8(string(data), contentErrorPreviewBytes)
}
