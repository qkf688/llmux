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

// resolveAuthType 仅 anthropic 且非空时返回指针；其它 type 或空字符串返回 nil（与现网 Create/Update 一致）。
func resolveAuthType(providerType, authType string) *string {
	if providerType == providers.TypeAnthropic && authType != "" {
		return &authType
	}
	return nil
}
