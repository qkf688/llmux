package adjustment

import (
	"github.com/qkf688/llmux/service/settings"
)

// settingsReader 是 adjustment 包的配置读取入口。
// 默认使用 settings.ModelsReader（语义等价 models.GetSetting*，零行为漂移）；
// 测试可通过 SetSettingsReader 注入 mock，传 nil 可复位默认。
var settingsReader settings.Reader = settings.ModelsReader

// SetSettingsReader 替换包级 Reader，供测试注入 mock 配置源。
// 生产路径无需调用；默认 ModelsReader 委托 models.GetSetting*。
// 传入 nil 时复位为 settings.ModelsReader，避免测试污染后无法恢复。
func SetSettingsReader(r settings.Reader) {
	if r == nil {
		settingsReader = settings.ModelsReader
		return
	}
	settingsReader = r
}
