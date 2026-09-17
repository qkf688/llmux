package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Stage 3 机械门（约定见 docs/guides/framework-conventions.md §5）：文档里的 rg
// 自查只有人跑才执法，这里把它编进 `go test ./...`——违反即红，执法不再靠自觉。
//
//   - G1：HTTP 500 出口唯一归 httpresp。禁止 c.JSON(500…) / http.Error / 直用
//     StatusInternalServerError 常量绕开收口（gin.Recovery 式的空 body 500 就是教训）。
//   - G2：httpresp.InternalErrorMessage 的 message 参数禁止拼接/取错误文本
//     （`+`、`.Error()`、`fmt.`）——静态文案入口一旦被塞进 err 原文，
//     Stage 3 的对外收口就从这一个参数漏掉。
func TestGate500NoLeakPaths(t *testing.T) {
	// 遍历（含空转哨兵）全部门共用，见 gate_sources_test.go——门扫不到文件就等于没执法，
	// 这类缺陷必须只修一次。
	files := gateSourceFiles(t, 250)

	// message 参数 = 第一个逗号后到下一个逗号前的片段；允许字面量与纯标识符（action、ie.Message）。
	reStaticCall := regexp.MustCompile(`httpresp\.InternalErrorMessage\(\w+,\s*([^,)]*)`)
	reJSON500 := regexp.MustCompile(`\.JSON\(\s*500\b`)
	reHTTPError500 := regexp.MustCompile(`http\.Error\(.*\b500\b`)
	var violations []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s: %v", f, err)
		}
		inHTTPResp := strings.HasPrefix(filepath.ToSlash(f), "httpresp/")
		for i, line := range strings.Split(string(src), "\n") {
			// G1：500 出口旁路
			if !inHTTPResp && (strings.Contains(line, "http.StatusInternalServerError") ||
				reJSON500.MatchString(line) ||
				reHTTPError500.MatchString(line)) {
				violations = append(violations, fmt.Sprintf("%s:%d 绕过 httpresp 手搓 500: %s", f, i+1, strings.TrimSpace(line)))
			}
			// G2：静态文案塞错误文本
			if m := reStaticCall.FindStringSubmatch(line); m != nil {
				arg := strings.TrimSpace(m[1])
				if strings.Contains(arg, "+") || strings.Contains(arg, ".Error()") || strings.Contains(arg, "fmt.") {
					violations = append(violations, fmt.Sprintf("%s:%d InternalErrorMessage 的 message 含错误文本: %s", f, i+1, strings.TrimSpace(line)))
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Errorf("500 收口质量门未通过（%d 处）:\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
