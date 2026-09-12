package testapi

import (
	"strings"
	"testing"
)

// TestParseProviderErrorDetail 与 TestGetErrorTypeFromStatus 已随实现收敛迁走：
// 细节提取 → common/upstream_detail_test.go，状态码映射 → common/errtype_test.go
// （单一来源在 common 直接测，不靠调用方委托测试间接覆盖）。

func TestBuildDetailedError(t *testing.T) {
	message := BuildDetailedError("network", "连接失败", "dial timeout", map[string]string{
		"provider": "openai",
		"model":    "gpt-4.1",
	})

	if !strings.Contains(message, "[网络错误] 连接失败") {
		t.Fatalf("unexpected header in message: %s", message)
	}
	if !strings.Contains(message, "详细信息: dial timeout") {
		t.Fatalf("unexpected detail in message: %s", message)
	}
	if !strings.Contains(message, "provider: openai") || !strings.Contains(message, "model: gpt-4.1") {
		t.Fatalf("unexpected context in message: %s", message)
	}
}
