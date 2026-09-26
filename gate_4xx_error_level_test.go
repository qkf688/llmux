package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Stage 25 机械门（约定见 docs/guides/framework-conventions.md §5「级别约定」）：
// **请求路径上的 4xx 客户端错误留痕 ≤ Warn**——ERROR 通道只留服务器故障（500 级）。
// 历史缺陷：handler/importexport 三处「slog.Error + httpresp.BadRequest」，一个持续发
// 坏请求（超大 body / 坏 JSON）的客户端就能把按 ERROR 计数的告警刷满，真故障被淹。
// 判据与 4xx 口径一致：slog.Error 与 4xx 响应出口（BadRequest / Unauthorized /
// NotFound / Forbidden / Conflict / Error / ErrorWithHttpStatus）并存于**同一个块**即红。
//
//   - 用「同一个块」而不是相邻行：与 gate_500_dual_log_test.go 同理，空行/注释/语句
//     重排不应成为绕过通道；
//   - slog.Warn 与 4xx 并存合法（正确级别，且 Warn 行是该失败的唯一留痕）；
//   - 后台任务 / 启动装配的 slog.Error 不调 httpresp，天然不在判据面（约定边界，见 §5）；
//   - 500 收口出口（InternalServerError*）不在本门判据面——那是 ERROR 的合法唯一入口，
//     由 gate_500_dual_log_test.go 管。
//
// 刻意边界（与 dual_log 门同类的固有局限，防误以为全量执法）：只认「同一块内的直接
// pkg.Func 调用」——handler 把 4xx 响应委托给同包 helper（helper 内部 slog.Error +
// BadRequest）即绕过本门；触发集只含**确定 4xx** 的具名出口（BadRequest/Unauthorized/
// NotFound/Forbidden/Conflict），httpresp.Error / ErrorWithHttpStatus 可承载任意状态码
// （/v1 上游透传 5xx 属合法 ERROR 留痕场景，见 framework-conventions §5 范围边界②），
// 刻意不进触发集。
func TestGate4xxClientErrorNotErrorLevel(t *testing.T) {
	files := gateSourceFiles(t, 250)

	var violations []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s: %v", f, err)
		}
		violations = append(violations, fourxxErrorLogSites(t, f, src)...)
	}
	if len(violations) > 0 {
		t.Errorf("4xx 级别门未通过（%d 处：客户端错误留痕占 ERROR 级）:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// fourxxErrorLogSites 找出「同一个块里 slog.Error 与 4xx 响应出口并存」的违规点。
func fourxxErrorLogSites(t *testing.T, path string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("解析 %s: %v", path, err)
	}

	var sites []string
	ast.Inspect(file, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		var slogErrPos, respPos token.Pos
		for _, stmt := range block.List {
			for _, call := range directCalls(stmt) {
				switch call.name {
				case "slog.Error":
					if !slogErrPos.IsValid() {
						slogErrPos = call.pos
					}
				case "httpresp.BadRequest", "httpresp.Unauthorized", "httpresp.NotFound",
					"httpresp.Forbidden", "httpresp.Conflict":
					if !respPos.IsValid() {
						respPos = call.pos
					}
				}
			}
		}
		if slogErrPos.IsValid() && respPos.IsValid() {
			sites = append(sites, fmt.Sprintf(
				"%s:%d 4xx 客户端错误留痕占 ERROR 级（服务器故障才配 ERROR；降 slog.Warn，内容保真）",
				path, fset.Position(slogErrPos).Line))
		}
		return true
	})
	return sites
}

// TestGate4xxClientErrorJudgment 判据自测：钉住不漏判与不误判（与
// TestGate500DualLogJudgment 同一用意）。
func TestGate4xxClientErrorJudgment(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "漏判风险-手写 ERROR 紧跟 4xx（历史写法，import.go 三处）",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"读取请求体失败\", \"error\", err)\n\t\thttpresp.BadRequest(c, \"读取请求体失败\")\n\t}\n}\n",
			want: true,
		},
		{
			name: "漏判风险-隔空行注释",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"x\", \"error\", err)\n\n\t\t// 说明\n\t\thttpresp.NotFound(c, \"x\")\n\t}\n}\n",
			want: true,
		},
		{
			name: "误判风险-ErrorWithHttpStatus 承载非 4xx（/v1 上游透传 5xx 合法 ERROR，刻意不进触发集）",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"x\", \"error\", err)\n\t\thttpresp.ErrorWithHttpStatus(c, 502, 502, \"x\")\n\t}\n}\n",
			want: false,
		},
		{
			name: "误判风险-Warn 级留痕 + 4xx（正确形态）",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Warn(\"x\", \"error\", err)\n\t\thttpresp.BadRequest(c, \"x\")\n\t}\n}\n",
			want: false,
		},
		{
			name: "误判风险-ERROR 紧跟 500 收口（归 dual_log 门管，不误伤）",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"x\", \"error\", err)\n\t\thttpresp.InternalErrorMessage(c, \"x\", err)\n\t}\n}\n",
			want: false,
		},
		{
			name: "误判风险-嵌套 if/else 两侧各一条",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tif a {\n\t\t\tslog.Error(\"x\", \"error\", err)\n\t\t} else {\n\t\t\thttpresp.BadRequest(c, \"x\")\n\t\t}\n\t}\n}\n",
			want: false,
		},
		{
			name: "误判风险-后台任务 ERROR 无 4xx 出口",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"x\", \"error\", err)\n\t\treturn\n\t}\n}\n",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(fourxxErrorLogSites(t, "sample.go", []byte(tc.src))) > 0
			if got != tc.want {
				t.Fatalf("判据结果 = %v, want %v（漏判则约束消失、误判则正常代码被误伤）", got, tc.want)
			}
		})
	}
}
