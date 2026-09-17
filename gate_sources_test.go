package main

// 机械门共用的源文件遍历（Stage 5，2026-09-17 抽出）。
//
// 为什么抽出来：机械门的全部价值建立在「真的扫到了源码」上，而目录过滤条件写错会让门
// 静默空转、永远绿——2026-09-17 发现 `gate_500_test.go` 从写下起就没扫到过任何一个文件：
// `filepath.WalkDir(".")` 的第一个回调就是遍历根，它的 `d.Name()` 恰好是 "."，被
// 「隐藏目录」判据 SkipDir 掉，整棵树当场被跳过。门各抄一份遍历 = 各抄一次这个坑。
//
// 因此这里同时提供三件防护：
//  1. 根目录不参与隐藏目录判据（`path != "."`）；
//  2. 数量下限哨兵（扫到的源文件数低于 minExpected 即红，防「漏扫半棵仓库」而不只是防扫空）；
//  3. 锚点文件断言（这几个必然存在的文件没被扫到，说明过滤条件又写错了）。

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// gateSourceAnchorFiles 遍历必须覆盖到的锚点（入口 + 三个层各一个），少一个即判空转。
var gateSourceAnchorFiles = []string{
	"main.go",
	"common/upstream_status.go",
	"service/chat/chat_attempt.go",
	"providers/provider.go",
}

// gateSourceFiles 列出待检源文件（非测试 .go），返回的路径一律 ToSlash 便于跨平台比对。
// 跳过：遍历根以外的点开头目录、以及顶层 webui / node_modules / db。
// 注意 webui/db 只在**顶层**跳过——按名字无条件跳曾让「未来出现同名包」静默漏扫。
func gateSourceFiles(t *testing.T, minExpected int) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(path)
		if d.IsDir() {
			if slash == "." {
				return nil
			}
			topLevel := !strings.Contains(slash, "/")
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if topLevel && (name == "webui" || name == "node_modules" || name == "db") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(slash, ".go") && !strings.HasSuffix(slash, "_test.go") {
			files = append(files, slash)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源文件失败: %v", err)
	}
	if len(files) < minExpected {
		t.Fatalf("扫描到 %d 个源文件，低于下限 %d——遍历被跳空或漏扫，本门正在空转", len(files), minExpected)
	}
	present := make(map[string]bool, len(files))
	for _, f := range files {
		present[f] = true
	}
	for _, anchor := range gateSourceAnchorFiles {
		if !present[anchor] {
			t.Fatalf("遍历未覆盖锚点文件 %s——目录过滤条件写错，本门扫不到它背后的整棵子树", anchor)
		}
	}
	return files
}
