package common

import (
	"encoding/json"
	"strings"
)

// CustomModelsConfigKey Provider.Config JSON 中「自定义模型清单」的键名。
// 单源引用此常量，禁止各包裸写字符串（modelsync 目录合并、channel 选路判定、
// channel.DropCustomModels 剥键三处共用）。
const CustomModelsConfigKey = "custom_models"

// ExtractCustomModels 从 Provider.Config JSON 提取自定义模型清单（管理员显式声明、
// 上游 /models 不返回的模型）。非法 JSON / 键缺失 / 元素非字符串均安全跳过；
// 空白条目不算成员（与白名单 CSV「空条目不算成员」同口径）。
//
// 判定集契约（2026-09-28 拍板）：自定义模型 = 供应商级声明授权，不受分组白名单约束，
// 该供应商全部分组可服务——消费方（modelsync 组织级目录、channel 选路/连通测试）不得
// 各自再写一份解析。
func ExtractCustomModels(config string) []string {
	if strings.TrimSpace(config) == "" {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return nil
	}
	raw, ok := parsed[CustomModelsConfigKey].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		id, ok := item.(string)
		if !ok {
			continue
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
