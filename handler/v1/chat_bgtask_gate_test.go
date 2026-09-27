package v1

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 机械门（Stage 30 AC-6）：handler 层禁止裸 go 启动后台 goroutine——会写库的后台任务
// 必须经 common/bgtask.Go 登记（AGENTS §6 清单项），裸 go 不在 bgtask.Shutdown 排空账
// 内，关机时正在写的请求记录会被关闭序的 models.Close() 硬切、失败留痕丢在建行占位上。
// 判据两条：① handler/ 树非测试 Go 文件零 ast.GoStmt；② chat.go 的 RecordLog 启动点
// 已走 bgtask.Go、原 WaitGroup 丝线已退场。改前实测红：chat.go 裸 go func 命中 ①。
func TestGateHandlerBgtaskRegistration(t *testing.T) {
	var bareGo []string
	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if gs, ok := n.(*ast.GoStmt); ok {
				bareGo = append(bareGo, fmt.Sprintf("%s:%d", path, fset.Position(gs.Pos()).Line))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk handler/: %v", err)
	}
	if len(bareGo) > 0 {
		t.Errorf("handler 层存在裸 go（会写库后台任务必须走 bgtask.Go 登记）:\n%s", strings.Join(bareGo, "\n"))
	}

	src, err := os.ReadFile("chat.go")
	if err != nil {
		t.Fatalf("read chat.go: %v", err)
	}
	if !strings.Contains(string(src), "bgtask.Go(") {
		t.Error("chat.go 的 RecordLog 后处理未走 bgtask.Go 登记（关机排空覆盖不到在写记录）")
	}
	if strings.Contains(string(src), "sync.WaitGroup") {
		t.Error("chat.go 仍带 WaitGroup 丝线：关机等待职责已移交 bgtask 排空，双轨并存会两套等待互相等")
	}
}
