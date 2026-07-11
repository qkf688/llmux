package testapi

import (
	"errors"

	"github.com/atopos31/llmio/providers"
)

var errInvalidProviderType = errors.New("invalid provider type")

// buildTestBody 从 provider 元数据取连通性测试体。
// 未知 type：返回 error（与 healthcheck 回退 OpenAI 策略不同）。
func buildTestBody(providerType string) ([]byte, error) {
	m, ok := providers.MetadataOf(providerType)
	if !ok || len(m.TestBody) == 0 {
		return nil, errInvalidProviderType
	}
	// 返回副本，避免调用方改写全局元数据
	out := make([]byte, len(m.TestBody))
	copy(out, m.TestBody)
	return out, nil
}

// buildStructuredOutputTestBody 从 provider 元数据取结构化输出测试体。
// 未知 type 或字段缺失：返回 error（与现网一致）。
func buildStructuredOutputTestBody(providerType string) ([]byte, error) {
	m, ok := providers.MetadataOf(providerType)
	if !ok || len(m.StructuredBody) == 0 {
		return nil, errInvalidProviderType
	}
	out := make([]byte, len(m.StructuredBody))
	copy(out, m.StructuredBody)
	return out, nil
}