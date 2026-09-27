package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/qkf688/llmux/common"
)

// Stage 26 回归：recovery 的 panic 留痕行 path attr 有长度上限（直达用例，
// 不依赖 httpresp 侧用例顺路覆盖）。path 本身超 4 KiB 时截断带统一标记——
// 该分支只记 URL.Path，长串必须放进 path 段。
func TestRecovery_LongPathAttrTruncated(t *testing.T) {
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	longPath := "/bomb/" + strings.Repeat("x", 8192)
	r := gin.New()
	r.Use(Recovery())
	// 无路由命中（Recovery 自留分支要求响应已提交，这里直接在 handler 里写满
	// 一个超长 path 的请求并 panic；任何 404 都会先经 NoRoute 而不 panic，
	// 故挂 NoRoute 上 panic）。
	r.NoRoute(func(c *gin.Context) {
		c.Writer.WriteString("partial") // 令响应已提交，走「自留一条」分支
		c.Writer.Flush()
		panic("late boom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, longPath, nil))

	line := logs.String()
	idx := strings.Index(line, "path=")
	if idx < 0 {
		t.Fatalf("panic 日志行缺 path attr:\n%s", line)
	}
	rest := line[idx+len("path="):]
	pathVal := rest
	if end := strings.Index(rest, " "); end >= 0 {
		pathVal = rest[:end]
	}
	if len(pathVal) > common.SlogPathMaxBytes+len(common.TruncatedSuffix) {
		t.Fatalf("path attr %d 字节，未按上限 %d 截断", len(pathVal), common.SlogPathMaxBytes)
	}
	if !strings.HasSuffix(pathVal, common.TruncatedSuffix) {
		t.Fatalf("截断必须带统一尾切标记，实际 path=%q", pathVal)
	}
	if !strings.HasPrefix(pathVal, `/bomb/`) {
		t.Fatalf("截断后应保留 path 前缀便于归因，实际 path=%q", pathVal)
	}
}
