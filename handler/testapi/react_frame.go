package testapi

import (
	"encoding/json"
	"fmt"
)

// frameMarshaler 帧数据序列化函数。生产用 marshalFrameJSON（json.Marshal 包装）；
// 测试注入必败实现以锁定停流契约——react 帧数据是固定结构体/字符串 map，json.Marshal
// 对它们现实不可失败，无法经输入构造失败场景，故走函数注入缝而非造输入
// （Stage 15 openai 侧立缝，Stage 30 anthropic 侧共用，两条 runner 不再各存一份口径）。
type frameMarshaler func(v any) (string, error)

func marshalFrameJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	return string(data), err
}

// marshalReactFrame 经注入缝序列化帧数据。失败错误即**停流信号**（两条 runner 同一份
// 口径）：调用方必须立即停止回写（失败帧自身与后续帧都不得进 SSE）并把错误回传，
// 由 handler 在流尾统一回 event:error，流内不插错误事件——禁止吞错续帧，吞错会让
// 排障时看到「error 判定已产生、流却还在输出」的混合事件序列。
func marshalReactFrame(marshal frameMarshaler, v any) (string, error) {
	data, err := marshal(v)
	if err != nil {
		return "", fmt.Errorf("react 帧数据序列化失败: %w", err)
	}
	return data, nil
}
