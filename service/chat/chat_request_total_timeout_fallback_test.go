package chat

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// fallingTotalTimeoutReader 测试用 mock：让 IntE 触发指定 err。
// 其余键（Reader.Bool/E/String/Int）一律返回 default——与生产 models.GetSetting* 同型。
// 失败路径与生产 models.GetSettingIntE 现有契约对齐（DB 故障 / 解析失败 / Min 越界三类 err
// 都被识别为读失败，调用方按 err 显式降级）。
type fallingTotalTimeoutReader struct {
	err error
}

func (fallingTotalTimeoutReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (fallingTotalTimeoutReader) Int(_ context.Context, _ string, def, _ int) int { return def }

func (fallingTotalTimeoutReader) String(_ context.Context, _ string, def string) string { return def }

func (fallingTotalTimeoutReader) BoolE(_ context.Context, _ string, def bool) (bool, error) {
	return def, nil
}

func (r fallingTotalTimeoutReader) IntE(_ context.Context, key string, def, _ int) (int, error) {
	if key == models.SettingKeyRequestTotalTimeout {
		return def, r.err
	}
	return def, nil
}

// captureFallbackLogs 注入 slog 默认 handler 捕获 LogReadSkip 输出，返回 buffer 与 cleanup。
// 同步清空 LogReadSkip 限频状态（跨用例 60 秒窗口会互相污染）。
func captureFallbackLogs(t *testing.T) *strings.Builder {
	t.Helper()
	settings.ResetReadSkipLimiterForTest()
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		settings.ResetReadSkipLimiterForTest()
	})
	return &logs
}

// TestGetRequestTotalTimeout_ReadFailure_FallsBackTo60sAndLogsSkip 锁住 stage37 AC-1 / AC-2：
// 当 request_total_timeout 读失败时（DB 故障 / 解析失败 / Min 越界三类 err），getRequestTotalTimeout
// 返回 conservativeRequestTotalTimeoutFallback / time.Second = 60，且 settings.LogReadSkip
// 留下"request total timeout conservative fallback"step 与底层 err。
//
// 变异自检验证——任一断言与实现绑死（注释掉 LogReadSkip 调用 / 改 60 → 200 / 改 return def）：
//   - 注释掉 LogReadSkip 调用 → "settings read failed" 子串缺失 → 红
//   - 改 60 → 200 → got != 60 命中红
//   - 改 return def（拿 default 走） → got == 600（schema 默认） ≠ 60 → 红
//   - err 为 nil（不模拟失败） → 不进 if err 分支、不会留痕 → 红
//
// 表驱动覆盖三个失败路径——与生产 models.GetSettingIntE 失败面同型。
func TestGetRequestTotalTimeout_ReadFailure_FallsBackTo60sAndLogsSkip(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		errMarker string // 断言日志里能看到的关键子串（避免 slog 转义双引号影响）
	}{
		{name: "DB failure", err: errors.New("simulated DB connection refused"), errMarker: "simulated DB connection refused"},
		{name: "parse failure", err: errors.New(`get setting request_total_timeout: invalid int value "not-a-number"`), errMarker: "invalid int value"},
		{name: "min boundary failure", err: errors.New("get setting request_total_timeout: value 0 below min 1"), errMarker: "below min 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureFallbackLogs(t)
			installChatTestSettings(t, fallingTotalTimeoutReader{err: tc.err})

			got := getRequestTotalTimeout(context.Background())

			if got != int(conservativeRequestTotalTimeoutFallback/time.Second) {
				t.Fatalf("getRequestTotalTimeout = %d, want %d (conservativeRequestTotalTimeoutFallback/Second)",
					got, int(conservativeRequestTotalTimeoutFallback/time.Second))
			}
			if !strings.Contains(logs.String(), "request total timeout conservative fallback") {
				t.Fatalf("LogReadSkip step label missing; got:\n%s", logs.String())
			}
			if !strings.Contains(logs.String(), "settings read failed") {
				t.Fatalf("LogReadSkip message prefix missing; got:\n%s", logs.String())
			}
			if !strings.Contains(logs.String(), tc.errMarker) {
				t.Fatalf("underlying err marker %q not propagated to log; got:\n%s", tc.errMarker, logs.String())
			}
		})
	}
}

// TestGetRequestTotalTimeout_ReadSuccess_UsesSchemaDefault 锁住 AC-3：
// 读成功（key 存在或未设置走 defaultValue=600）时 getRequestTotalTimeout 返回 600，行为不变。
//
// 变异自检验证：
//   - 把 fallback 路径的 return 60 换成 60s 以外的数 → 此测试不依赖 fallback，断言仍绿（证
//     明 AC-3 与 AC-1/AC-2 互不污染：失败路径改坏不影响成功路径测试）
func TestGetRequestTotalTimeout_ReadSuccess_UsesSchemaDefault(t *testing.T) {
	logs := captureFallbackLogs(t)
	installChatTestSettings(t, fallingTotalTimeoutReader{err: nil}) // 不触发失败

	got := getRequestTotalTimeout(context.Background())

	if got != 600 {
		t.Fatalf("getRequestTotalTimeout = %d, want 600 (schema default)", got)
	}
	if strings.Contains(logs.String(), "settings read failed") {
		t.Fatalf("LogReadSkip must not fire on read success; got:\n%s", logs.String())
	}
}
