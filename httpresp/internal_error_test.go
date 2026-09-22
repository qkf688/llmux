package httpresp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/qkf688/llmux/consts"
)

func init() { gin.SetMode(gin.TestMode) }

// captureSlog 把 slog 默认输出重定向到 buffer，返回 buffer 供断言日志侧内容。
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// newCtx 构造带请求编号的 gin 上下文（模拟已过 RequestID 中间件）。
// 注入值一律带来源前缀（Stage 17 生产形态：gw: 自生成 / in: 沿用）——httpresp 对
// rid 不透明（拿到什么拼什么），fixture 与生产同形防后来者误读。
func newCtx(rid string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/pools", nil)
	if rid != "" {
		c.Set(consts.ContextKeyRequestID, rid)
	}
	return c, w
}

const leakyErrText = `UNIQUE constraint failed: api_keys.key_hash`

// 契约（Stage 3）：InternalServerError 收到 error 后——
// ① 对外 HTTP 500，message/error 两字段为同一句内嵌编号的通用文案，原始错误不得出现在响应体；
// ② 对内 slog.Error 一行，含原始错误、request_id、method、path。
func TestInternalServerError_GenericResponseAndServerSideTrace(t *testing.T) {
	logs := captureSlog(t)
	c, w := newCtx("gw:a1b2c3d4e5f6")

	InternalServerError(c, errors.New(leakyErrText))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("HTTP 状态 = %d, want 500", w.Code)
	}
	body := w.Body.String()
	want := "internal server error [req=gw:a1b2c3d4e5f6]"
	if got := gjson.Get(body, "message").String(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got := gjson.Get(body, "error").String(); got != want {
		t.Errorf("error 字段 = %q, want 与 message 同句", got)
	}
	if got := gjson.Get(body, "code").Int(); got != 500 {
		t.Errorf("code = %d, want 500", got)
	}
	if strings.Contains(body, leakyErrText) || strings.Contains(body, "api_keys") {
		t.Errorf("响应体泄露原始错误:\n%s", body)
	}

	line := logs.String()
	for _, frag := range []string{leakyErrText, "request_id=gw:a1b2c3d4e5f6", "method=POST", "path=/api/pools", "level=ERROR"} {
		if !strings.Contains(line, frag) {
			t.Errorf("日志行缺片段 %q，实际:\n%s", frag, line)
		}
	}
}

// 中间件缺席（单测直连 httpresp）时文案形状稳定：无编号则不拼 [req=] 后缀。
func TestInternalServerError_WithoutRequestID(t *testing.T) {
	captureSlog(t)
	c, w := newCtx("")

	InternalServerError(c, errors.New("disk on fire"))

	if got := gjson.Get(w.Body.String(), "message").String(); got != "internal server error" {
		t.Errorf("message = %q, want 恒定文案无后缀", got)
	}
}

// 静态文案入口：对外原样返回已审查文案（不拼编号），对内仍留痕（含编号、message
// 与原始 err——同名分支多，message 是日志里区分「哪一处」的坐标）。
func TestInternalErrorMessage_KeepsVettedMessageButLogs(t *testing.T) {
	logs := captureSlog(t)
	c, w := newCtx("gw:f6e5d4c3b2a1")

	InternalErrorMessage(c, "encryption not configured", errors.New("cipher is nil"))

	if got := gjson.Get(w.Body.String(), "message").String(); got != "encryption not configured" {
		t.Errorf("message = %q, want 静态文案原样", got)
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP 状态 = %d, want 500", w.Code)
	}
	line := logs.String()
	for _, frag := range []string{"cipher is nil", "request_id=gw:f6e5d4c3b2a1", `message="encryption not configured"`} {
		if !strings.Contains(line, frag) {
			t.Errorf("日志行缺片段 %q，实际:\n%s", frag, line)
		}
	}
}

// 客户端断连/上传中断不是服务器故障：降 Warn，防止「建连即断」刷满 ERROR 告警通道。
func TestInternalServerError_ClientDisconnectDowngradesToWarn(t *testing.T) {
	logs := captureSlog(t)
	c, w := newCtx("gw:9f8e7d6c5b4a")

	InternalServerError(c, fmt.Errorf("read body: %w", context.Canceled))

	// 对外形状不变（仍然 500 + 编号），只有日志级别降级。
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP 状态 = %d, want 500", w.Code)
	}
	line := logs.String()
	if !strings.Contains(line, "level=WARN") {
		t.Errorf("断连类错误应为 WARN，实际:\n%s", line)
	}
	if !strings.Contains(line, "request_id=gw:9f8e7d6c5b4a") {
		t.Errorf("降级不得丢掉留痕字段:\n%s", line)
	}
}

// 真故障必须保持 ERROR（降级判定不许吞掉正常告警）。
func TestInternalServerError_RealFaultStaysError(t *testing.T) {
	logs := captureSlog(t)
	c, _ := newCtx("gw:112233445566")

	InternalServerError(c, errors.New("db is locked"))

	if line := logs.String(); !strings.Contains(line, "level=ERROR") {
		t.Errorf("非断连故障应维持 ERROR:\n%s", line)
	}
}

func TestInternalErrorMessage_NilErr(t *testing.T) {
	logs := captureSlog(t)
	c, w := newCtx("gw:x1x2x3x4x5x6")

	InternalErrorMessage(c, "failed to sign token", nil)

	if !gjson.Get(w.Body.String(), "message").Exists() {
		t.Fatal("message 字段缺失")
	}
	line := logs.String()
	if !strings.Contains(line, "request_id=gw:x1x2x3x4x5x6") {
		t.Errorf("日志行缺编号:\n%s", line)
	}
	if strings.Contains(line, "error=") {
		t.Errorf("nil err 不应写出 error 字段:\n%s", line)
	}
}

// logRecords 把 TextHandler 输出切成逐条记录（一条记录占一行）。
func logRecords(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Stage 18 契约：一次 500 故障只产**一条**日志记录，业务上下文（credential_id 等）
// 经 attrs 并进同一行。此前调用点常「先手写一行细节、收口再留一行错误」——同一次
// 故障占两条 ERROR，按行计数的告警翻倍，且细节行得靠 rid 才能与错误行对上。
// 本用例同时锁「留痕条数」与「细节与编号同处一行」，任何回到双份留痕的改法都红。
func TestInternalErrorMessage_BusinessAttrsFoldIntoSingleRecord(t *testing.T) {
	logs := captureSlog(t)
	c, w := newCtx("gw:0a1b2c3d4e5f")

	InternalErrorMessage(c, "Failed to decrypt key", errors.New("cipher: bad padding"),
		"credential_id", uint(42), "group_id", uint(7))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("HTTP 状态 = %d, want 500", w.Code)
	}
	records := logRecords(logs.String())
	if len(records) != 1 {
		t.Fatalf("一次 500 故障写了 %d 条日志记录，want 1（双份留痕会让按 ERROR 计数的告警翻倍）:\n%s", len(records), logs.String())
	}
	for _, frag := range []string{
		"level=ERROR",
		"request_id=gw:0a1b2c3d4e5f",
		`message="Failed to decrypt key"`,
		"credential_id=42",
		"group_id=7",
		"cipher: bad padding",
	} {
		if !strings.Contains(records[0], frag) {
			t.Errorf("唯一记录缺片段 %q，实际:\n%s", frag, records[0])
		}
	}
}

// 同一口径对通用出口同样成立（Stage 18）。
func TestInternalServerError_SingleRecordWithAttrs(t *testing.T) {
	logs := captureSlog(t)
	c, _ := newCtx("gw:998877665544")

	InternalServerError(c, errors.New("boom"), "stack", "goroutine 1 [running]:")

	records := logRecords(logs.String())
	if len(records) != 1 {
		t.Fatalf("一次 500 故障写了 %d 条日志记录，want 1:\n%s", len(records), logs.String())
	}
	if !strings.Contains(records[0], "stack=") {
		t.Errorf("业务 attrs 未进同一行:\n%s", records[0])
	}
}
