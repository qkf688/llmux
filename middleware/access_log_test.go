package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
)

// Stage 16 契约（访问行单行上限）：整行 ≤ accessLogLineMaxBytes（4 KiB），超限时
// path 段被截断并带 common.TruncatedSuffix 标记，status/耗时/rid 恒完整存活
// （方案 B：截断施加在 %q 之后的 path 段，排障关联键不随病态 URL 丢失）；
// 截断点落在多字节 UTF-8 中间时回退 rune 边界，行内无 U+FFFD、恒单 \n。

func accessLogTestParams(path, rid string) gin.LogFormatterParams {
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set(consts.HeaderRequestID, rid)
	return gin.LogFormatterParams{
		TimeStamp:  time.Now(),
		StatusCode: 200,
		Latency:    time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     http.MethodGet,
		Path:       path,
		Request:    req,
	}
}

func assertTruncatedLineShape(t *testing.T, line string) {
	t.Helper()
	if len(line) > accessLogLineMaxBytes {
		t.Fatalf("访问行 %d 字节 > 上限 %d: %q", len(line), accessLogLineMaxBytes, line)
	}
	if strings.Count(line, "\n") != 1 {
		t.Fatalf("访问行必须恒单行: %q", line)
	}
	if !strings.Contains(line, "rid=abc123") {
		t.Fatalf("rid 被截掉（排障关联键必须恒存活）: %q", line)
	}
	if !strings.Contains(line, "| 200 |") {
		t.Fatalf("status 段被截掉: %q", line)
	}
	if !strings.Contains(line, "ms") {
		t.Fatalf("耗时段被截掉（AC-3 三个关联键恒存活）: %q", line)
	}
	if !strings.Contains(line, common.TruncatedSuffix) {
		t.Fatalf("超限行缺截断标记: %q", line)
	}
}

func TestFormatAccessLine_OverlongClientIPCapsWholeLine(t *testing.T) {
	// 审查回批 W1：gin 默认信任全部代理且本项目未 SetTrustedProxies（Stage 20 议题），
	// XFF 伪造的 ClientIP 无长度约束——path 段预算压不下时必须整行兜底，
	// 体量上界是防线本体（rid/status 在载体字段病态时被切是定案取舍）。
	p := accessLogTestParams("/probe", "abc123")
	p.ClientIP = strings.Repeat("9", 8000)
	line := formatAccessLine(p)
	if len(line) > accessLogLineMaxBytes {
		t.Fatalf("病态 ClientIP 下访问行 %d 字节 > 上限 %d（整行兜底缺失）", len(line), accessLogLineMaxBytes)
	}
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("兜底行必须保持「单行 + 尾换行」形态（否则下一条访问行会拼接进来）: %q", line)
	}
	if !strings.Contains(line, common.TruncatedSuffix) {
		t.Fatalf("兜底行缺截断标记: %q", line)
	}
}

func TestFormatAccessLine_TruncatesOverlongASCIIPath(t *testing.T) {
	line := formatAccessLine(accessLogTestParams("/"+strings.Repeat("a", 8000), "abc123"))
	assertTruncatedLineShape(t, line)
}

func TestFormatAccessLine_TruncatesCJKPathOnRuneBoundary(t *testing.T) {
	// 3 字节字符填充：字节预算几乎必然劈在多字节中间，rune 边界回退必须生效。
	line := formatAccessLine(accessLogTestParams("/"+strings.Repeat("中", 3000), "abc123"))
	assertTruncatedLineShape(t, line)
	if !utf8.ValidString(line) || strings.Contains(line, "\uFFFD") {
		t.Fatalf("截断把 UTF-8 劈半产生乱码: %q", line)
	}
}

func TestFormatAccessLine_TruncatesCJKQuerySafely(t *testing.T) {
	// RawQuery 按客户端发送的字节原样进 path 段，可含未编码 UTF-8（搜索词类参数）。
	path := "/" + strings.Repeat("搜", 1500) + "?" + strings.Repeat("词", 1500)
	line := formatAccessLine(accessLogTestParams(path, "abc123"))
	assertTruncatedLineShape(t, line)
	if !utf8.ValidString(line) || strings.Contains(line, "\uFFFD") {
		t.Fatalf("截断把 UTF-8 劈半产生乱码: %q", line)
	}
}

func TestFormatAccessLine_UnderLimitLineUnchanged(t *testing.T) {
	// 常规请求回归：短行不得带截断标记，%q 引号形态保持。
	line := formatAccessLine(accessLogTestParams("/probe?k=v", "abc123"))
	if strings.Contains(line, common.TruncatedSuffix) {
		t.Fatalf("短行误带截断标记: %q", line)
	}
	if !strings.Contains(line, `"/probe?k=v"`) {
		t.Fatalf("path %q 引号形态被破坏: %q", `"/probe?k=v"`, line)
	}
	if !strings.Contains(line, "rid=abc123") {
		t.Fatalf("短行缺 rid: %q", line)
	}
}

// 回归（Stage 3 审查 P1-2，自 recovery_test.go 顺迁）：gin 默认 formatter 用 %#v 打印
// path 天然免疫换行注入；自定义 formatter 若裸 %s，百分号编码的 %0A 解码成真换行后可
// 伪造整行访问记录。判据：任意 path 一行请求只许产出一行日志。
func TestFormatAccessLine_SingleLineAgainstNewlineInjection(t *testing.T) {
	injected := "/x\n[GIN] 2026/09/17 - 00:00:00 | 200 | 1ms | 1.2.3.4 | GET forged | rid=deadbeef"
	line := formatAccessLine(accessLogTestParams(injected, "abc123"))
	if strings.Count(line, "\n") != 1 {
		t.Errorf("换行注入把访问行劈成多行:\n%q", line)
	}
	if !strings.Contains(line, "rid=abc123") {
		t.Errorf("访问行缺 rid: %q", line)
	}
}
