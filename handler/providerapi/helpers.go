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

// resolveAuthType 顶层 auth_type 列的入库判定：**仅 anthropic** 且值 ∈ {bearer,
// x-api-key} 时收录；其它 type（含 openai 系）或空/未知值返回 nil。
// openai 系的认证方式不落本列——它存在 config 的 openai_auth_type 键（运行时真值
// 永远在 config JSON，列只是 anthropic 的展示副本；混入 openai 值会造成
// 「列显 bearer、实际发 x-api-key」的假象，审查 P0 拆键裁决后口径收紧）。
func resolveAuthType(providerType, authType string) *string {
	if providerType != providers.TypeAnthropic {
		return nil
	}
	if authType != providers.AuthTypeBearer && authType != providers.AuthTypeXAPIKey {
		return nil
	}
	v := authType
	return &v
}
