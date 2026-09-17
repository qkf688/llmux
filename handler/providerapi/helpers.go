package providerapi

import (
	"strconv"

	"github.com/qkf688/llmux/providers"
)

func parseUintParam(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

// resolveAuthType 认证方式入库判定（Stage 4b）：anthropic 与 openai 系都收
// bearer / x-api-key 两枚举值；空值或未知值返回 nil（装配 provider 时回退各自
// 类型默认：anthropic=x-api-key、openai 系=bearer）。其它 provider type 不识别
// 该字段，一律 nil。
func resolveAuthType(providerType, authType string) *string {
	if authType != providers.AuthTypeBearer && authType != providers.AuthTypeXAPIKey {
		return nil
	}
	switch providerType {
	case providers.TypeAnthropic, providers.TypeOpenAI, providers.TypeOpenAIRes:
		v := authType
		return &v
	}
	return nil
}
