package httpresp

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qkf688/llmux/common"
)

// Stage 26 回归：500 留痕行的 path attr 有长度上限——病态长 URL 触发 500 时
// slog 行不再被撑到 MiB 级（与访问行 4 KiB 截断同族同值，访问行已锁、本测试锁
// slog 侧）。两处 path 写点（httpresp logInternalError + middleware recovery）
// 各一条直达用例，不靠顺路覆盖。

func TestLogInternalError_LongPathTruncated(t *testing.T) {
	logs := captureSlog(t)
	w := httptest.NewRecorder()
	c, _ := ginTestContextWithLongPath(w)

	InternalServerError(c, errLongPathFixture())

	line := logs.String()
	if strings.Count(line, "path=") == 0 {
		t.Fatalf("日志行缺 path attr:\n%s", line)
	}
	pathVal := pathAttrValue(line)
	if len(pathVal) > slogPathLimit+len(common.TruncatedSuffix) {
		t.Fatalf("path attr %d 字节，未按上限 %d 截断", len(pathVal), slogPathLimit)
	}
	if !strings.HasSuffix(pathVal, common.TruncatedSuffix) {
		t.Fatalf("截断必须带统一尾切标记，实际 path=%q", pathVal)
	}
	if !strings.HasPrefix(pathVal, "/bomb") {
		t.Fatalf("截断后应保留 path 前缀便于归因，实际 path=%q", pathVal)
	}
}

// pathAttrValue 从 slog 文本行提取 path= 的值（单行单 path 假设，本测试 fixture 成立）。
func pathAttrValue(line string) string {
	idx := strings.Index(line, "path=")
	if idx < 0 {
		return ""
	}
	rest := line[idx+len("path="):]
	if end := strings.Index(rest, " "); end >= 0 {
		return rest[:end]
	}
	return strings.TrimRight(rest, "\n")
}
