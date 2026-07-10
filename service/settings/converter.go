package settings

import (
	"github.com/atopos31/llmio/models"
)

// ToString 将任意类型的值按 SettingType 序列化为字符串。
func ToString(value any, t models.SettingType) (string, error) {
	return models.SettingValueToString(value, t)
}

// FromString 将字符串按 SettingType 反序列化为任意类型。
func FromString(value string, t models.SettingType) (any, error) {
	return models.SettingValueFromString(value, t)
}
