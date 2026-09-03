package providerapi

import (
	"encoding/json"
)

// dropCustomModels 从 config JSON 中移除 custom_models 字段。
// 上游浏览场景只构造临时 config（喂 providers.New，不落库），与 modelsync 版
// 差异在只剥 custom_models——upstream_models 是遗留死键、无运行时消费者，
// 残留在临时 config 无害（存量数据由 models 启动迁移统一清理）。
func dropCustomModels(config string) (string, error) {
	parsed, err := parseConfigMap(config)
	if err != nil {
		return "", err
	}
	delete(parsed, "custom_models")
	return marshalConfigMap(parsed)
}

// sanitizeConfig 清洗 Create/Update 写入的 config：去掉已迁出结构化字段的
// api_key / _schedule，只保留 adapter 字段（base_url / beta / version / custom_models 等）。
func sanitizeConfig(config string) (string, error) {
	parsed, err := parseConfigMap(config)
	if err != nil {
		return "", err
	}
	delete(parsed, "api_key")
	delete(parsed, "_schedule")
	return marshalConfigMap(parsed)
}

func parseConfigMap(config string) (map[string]any, error) {
	if config == "" {
		return map[string]any{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		return map[string]any{}, nil
	}
	return parsed, nil
}

func marshalConfigMap(parsed map[string]any) (string, error) {
	updated, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(updated), nil
}
