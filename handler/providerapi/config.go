package providerapi

import (
	"encoding/json"
)

// dropCustomModels 从 config JSON 中移除 custom_models 字段。
// 注意：与 modelsync 内部实现不同——此处故意不删 upstream_models，
// 以保持 GetProviderModels?source=upstream 的现网语义。
func dropCustomModels(config string) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", err
	}

	delete(parsed, "custom_models")

	updated, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(updated), nil
}
