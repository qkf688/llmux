package main

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// gateSourceFiles 供所有机械门共用的源文件遍历（列出待检的非测试 .go 文件）。
//
// 为什么抽出来：机械门的全部价值建立在「真的扫到了源码」上，而目录过滤条件写错会让门
// 静默空转、永远绿——2026-09-17 就发现 `gate_500_test.go` 从写下起第一步就把遍历根目录
// "."（名字以 "." 开头）当隐藏目录 SkipDir 掉了，一个文件都没扫。这类缺陷只在共用实现里
// 修一次、加一次哨兵才守得住；各门各抄一份遍历 = 各抄一次踩坑机会。
//
// minExpected 是仓库规模下限的粗哨兵（按当前源码文件数留足余量），遍历被跳空时它先红。
func gateSourceFiles(t *testing.T, minExpected int) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// path != "." 不可省：遍历根目录自身的名字就是 "."，省了会把整棵树跳空。
			if path != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "webui" || d.Name() == "node_modules" || d.Name() == "db") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源文件失败: %v", err)
	}
	if len(files) < minExpected {
		t.Fatalf("扫描到 %d 个源文件，明显低于仓库规模下限 %d——遍历被跳空，本门正在空转", len(files), minExpected)
	}
	return files
}
