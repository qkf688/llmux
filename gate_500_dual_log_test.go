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

// Stage 18 机械门（约定见 docs/guides/framework-conventions.md §5「ERROR 唯一入口」）：
// 一次 500 故障只允许留一条 ERROR，且只能出自 httpresp 的收口出口。历史上调用点常
// 「手写 slog.Error/Warn 一行业务细节 + 收口再留一行错误」——同一次故障占两条 ERROR，
// 按行计数的告警翻倍。业务细节已改为经收口出口的 attrs 并进同一行（其契约测试在
// httpresp/internal_error_test.go），本门负责拦住写法回流。
//
// 判据（AST 级，不是文本行匹配）：**同一个块**里同时出现 slog.Error / slog.Warn 与
// httpresp.InternalServerError / httpresp.InternalErrorMessage 即判红。
//
//   - 用「同一个块」而不是「相邻两行」：后者会被空行、注释、语句重排绕过；
//   - 用 AST 而不是数大括号：字符串与注释里的 `{` 会把文本计数带偏；
//   - 不下钻嵌套块：`if a { slog… } else { httpresp… }` 是两个分支、不是双份留痕；
//     `handler/logs` 的「ChatIO 删除失败记 Warn、ChatLog 删除失败走收口」同理——
//     两条留痕针对两个错误，各在各的 if 块里。
//
// 边界（刻意不管的两类，别当漏判）：① 4xx 客户端错误的手写留痕（httpresp.BadRequest
// 不留痕，那行是该次失败的唯一记录，删了线索就没了）；② 后台任务 / 启动装配的
// slog.Error（没有请求上下文，本就没有收口出口可走）。
func TestGate500SingleErrorRecordPerFailure(t *testing.T) {
	// 遍历（含空转哨兵）全部门共用，见 gate_sources_test.go。
	files := gateSourceFiles(t, 250)

	var violations []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s: %v", f, err)
		}
		violations = append(violations, dualLogSites(t, f, src)...)
	}
	if len(violations) > 0 {
		t.Errorf("500 留痕唯一入口门未通过（%d 处双份留痕）:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// namedCall 一条语句里的包级调用（pkg.Func 形态）及其源位置。
type namedCall struct {
	name string
	pos  token.Pos
}

// directCalls 列出一条语句里的全部包级调用，不下钻嵌套块（嵌套块是另一个分支）。
func directCalls(stmt ast.Stmt) []namedCall {
	var out []namedCall
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BlockStmt:
			return false
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			out = append(out, namedCall{name: ident.Name + "." + sel.Sel.Name, pos: node.Pos()})
		}
		return true
	})
	return out
}

// dualLogSites 找出「同一个块里 slog.Error/Warn 与 httpresp.InternalError* 并存」的
// 双份留痕点，返回带 file:line 的违规描述。
func dualLogSites(t *testing.T, path string, src []byte) []string {
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
		var slogPos, respPos token.Pos
		for _, stmt := range block.List {
			for _, call := range directCalls(stmt) {
				switch call.name {
				case "slog.Error", "slog.Warn":
					if !slogPos.IsValid() {
						slogPos = call.pos
					}
				case "httpresp.InternalServerError", "httpresp.InternalErrorMessage":
					if !respPos.IsValid() {
						respPos = call.pos
					}
				}
			}
		}
		if slogPos.IsValid() && respPos.IsValid() {
			sites = append(sites, fmt.Sprintf(
				"%s:%d 双份留痕：同一次失败在 slog 与 httpresp 收口各记一次（业务细节请走收口的 attrs 参数）",
				path, fset.Position(slogPos).Line))
		}
		return true
	})
	return sites
}

// TestGate500DualLogJudgment 判据自测：钉住不漏判（三种真实历史写法）与不误判
// （嵌套分支、4xx 独行留痕、正常收口），防门本身退化成空转或误伤正常代码——
// 与 gate_upstream_status_test.go 的判据样例测试同一用意。
func TestGate500DualLogJudgment(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "漏判风险-手写行紧接收口行（历史写法）",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"x\", \"error\", err)\n\t\thttpresp.InternalErrorMessage(c, \"x\", err)\n\t}\n}\n",
			want: true,
		},
		{
			name: "漏判风险-两行之间隔着空行与注释",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Warn(\"x\", \"error\", err)\n\n\t\t// 说明\n\t\thttpresp.InternalServerError(c, err)\n\t}\n}\n",
			want: true,
		},
		{
			name: "漏判风险-业务细节带额外 attrs 的历史写法",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Warn(\"x\", \"credential_id\", id, \"error\", err)\n\t\thttpresp.InternalErrorMessage(c, \"x\", err)\n\t}\n}\n",
			want: true,
		},
		{
			name: "误判风险-嵌套 if/else 两侧各一条",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tif a {\n\t\t\tslog.Warn(\"x\", \"error\", err)\n\t\t} else {\n\t\t\thttpresp.InternalServerError(c, err)\n\t\t}\n\t}\n}\n",
			want: false,
		},
		{
			name: "误判风险-4xx 客户端错误的独行留痕",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\tslog.Error(\"x\", \"error\", err)\n\t\thttpresp.BadRequest(c, \"x\")\n\t}\n}\n",
			want: false,
		},
		{
			name: "误判风险-只有收口留痕",
			src:  "package p\n\nfunc f() {\n\tif err != nil {\n\t\thttpresp.InternalServerError(c, err)\n\t}\n}\n",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(dualLogSites(t, "sample.go", []byte(tc.src))) > 0
			if got != tc.want {
				t.Fatalf("判据结果 = %v, want %v（漏判则约束消失、误判则正常代码被误伤）", got, tc.want)
			}
		})
	}
}
