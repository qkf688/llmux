package chat

import (
	"encoding/json"
	"fmt"

	"github.com/qkf688/llmux/consts"
)

// StreamErrorWriter 生成「提交后断流」的协议内 SSE error 事件整帧字节（event/data 行
// 与分隔空行一次给出）。按 style 注册（与 Beforer/Processer 同键同节奏，Stage 30 W5：
// 原 handler 层 style switch 是新 style 的静默漏点，收进注册面后 handler 只查表搬运、
// 不认任何协议形状）。
//
// 契约：
//   - message 必须是静态文案（禁拼内部错误文本，framework-conventions §5）；
//   - 帧内 JSON 由 typed 结构体 + json.Marshal 兜底转义，禁止字符串拼接造 JSON；
//   - 注册完备性有机械锁（stream_error_writer_test.go：凡注册了 Beforer 的 style 必须
//     有本配方，漏登记测试即红），不再依赖人记得补 switch 分支。
type StreamErrorWriter func(message string) ([]byte, error)

// 断流错误帧的三种形状（客户端 SDK 能否识别失败的关键契约，字节形状由
// stream_error_writer_test.go 逐字锁定）：
//   - anthropic / openai-res：event: error 行 + 错误对象；
//   - openai：data: 行 + error 对象。
//
// 形状都不再跟随终止事件（错误事件即流尾；各家 SDK 对「无终止事件的流」如何收尾
// 未经真机验证，见 framework-conventions §6 已知边界）。字段顺序即帧内 JSON 键序，
// 与既有字节形状逐字对齐，调整字段顺序会打破客户端契约，勿随意重排。

// streamErrorAnthropicFrame anthropic 错误帧（api_error 而非 overloaded_error：
// 断流不是过载，overloaded_error 会被 Anthropic SDK 判为可重试并整请求重放）。
type streamErrorAnthropicFrame struct {
	Type  string `json:"type"` // 恒 "error"
	Error struct {
		Type    string `json:"type"` // 恒 "api_error"
		Message string `json:"message"`
	} `json:"error"`
}

// streamErrorOpenAIFrame OpenAI Chat 错误帧（data: 行 + error 对象）。
type streamErrorOpenAIFrame struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`  // 恒 "server_error"
		Param   any    `json:"param"` // 恒 null（键必须在，禁 omitempty）
		Code    int    `json:"code"`  // 恒 500
	} `json:"error"`
}

// streamErrorOpenAIResFrame Responses 错误帧（event: error 行 + 错误对象）。
type streamErrorOpenAIResFrame struct {
	Type  string `json:"type"` // 恒 "error"
	Error struct {
		Code    string `json:"code"` // 恒 "server_error"
		Message string `json:"message"`
	} `json:"error"`
}

func init() {
	RegisterStreamErrorWriter(string(consts.StyleAnthropic), streamErrorWriterAnthropic)
	RegisterStreamErrorWriter(string(consts.StyleOpenAI), streamErrorWriterOpenAI)
	RegisterStreamErrorWriter(string(consts.StyleOpenAIRes), streamErrorWriterOpenAIRes)
}

func streamErrorWriterAnthropic(message string) ([]byte, error) {
	frame := streamErrorAnthropicFrame{Type: "error"}
	frame.Error.Type = "api_error"
	frame.Error.Message = message
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic stream error event: %w", err)
	}
	return []byte("event: error\ndata: " + string(data) + "\n\n"), nil
}

func streamErrorWriterOpenAI(message string) ([]byte, error) {
	frame := streamErrorOpenAIFrame{}
	frame.Error.Message = message
	frame.Error.Type = "server_error"
	frame.Error.Code = 500
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("marshal openai stream error event: %w", err)
	}
	return []byte("data: " + string(data) + "\n\n"), nil
}

func streamErrorWriterOpenAIRes(message string) ([]byte, error) {
	frame := streamErrorOpenAIResFrame{Type: "error"}
	frame.Error.Code = "server_error"
	frame.Error.Message = message
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("marshal openai-res stream error event: %w", err)
	}
	return []byte("event: error\ndata: " + string(data) + "\n\n"), nil
}
