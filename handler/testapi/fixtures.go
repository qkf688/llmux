package testapi

import (
	"errors"

	"github.com/qkf688/llmux/providers"
)

var errInvalidProviderType = errors.New("invalid provider type")

// buildTestBodyFrom 是各测试体构建函数的共同骨架：按 pick 从 provider 元数据取对应字段，
// 未知 type 或字段缺失返回 errInvalidProviderType（与 healthcheck 回退 OpenAI 策略不同）。
// 返回副本：调用方（BuildReq 的 sjson 注入路径）不改全局元数据（元数据是包级共享状态，改脏会扩散到所有链路）。
func buildTestBodyFrom(providerType string, pick func(providers.Metadata) []byte) ([]byte, error) {
	m, ok := providers.MetadataOf(providerType)
	body := pick(m)
	if !ok || len(body) == 0 {
		return nil, errInvalidProviderType
	}
	out := make([]byte, len(body))
	copy(out, body)
	return out, nil
}

// buildTestBody 取连通性测试体。
func buildTestBody(providerType string) ([]byte, error) {
	return buildTestBodyFrom(providerType, func(m providers.Metadata) []byte { return m.TestBody })
}

// buildStructuredOutputTestBody 取结构化输出测试体。
func buildStructuredOutputTestBody(providerType string) ([]byte, error) {
	return buildTestBodyFrom(providerType, func(m providers.Metadata) []byte { return m.StructuredBody })
}

// buildThinkingTestBody 取 thinking 能力测试体。
func buildThinkingTestBody(providerType string) ([]byte, error) {
	return buildTestBodyFrom(providerType, func(m providers.Metadata) []byte { return m.ThinkingBody })
}

// buildVisionTestBody 取图片识别能力测试体。
func buildVisionTestBody(providerType string) ([]byte, error) {
	return buildTestBodyFrom(providerType, func(m providers.Metadata) []byte { return m.VisionBody })
}
