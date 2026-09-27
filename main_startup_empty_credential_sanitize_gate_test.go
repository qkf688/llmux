package main

import (
	"os"
	"strings"
	"testing"
)

// Stage 27 机械门：存量空明文凭据清洗（DeleteLegacyEmptyKeyCredentials）与 chat_logs
// 头快照清洗同属启动期一次性清洗，必须早于 startBackgroundServices(ctx)——清洗时刻
// 无凭据并发写者（credwrite worker / 健康检查 / 探活都未启动）。挪到之后会与凭据
// 状态机写路径交错（行被读入判空后、Delete 前恰好被冷却/判停写库，删除窗口基于旧值
// 判定）；删掉调用则存量空明文行继续在后台显示 active 误导管理员。
// 判据结构同 main_startup_sanitize_gate_test.go。
func TestGateStartupEmptyCredentialSanitizeRunsBeforeBackgroundServices(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读取 main.go: %v", err)
	}
	content := string(src)
	sanitize := strings.Index(content, "DeleteLegacyEmptyKeyCredentials")
	background := strings.Index(content, "startBackgroundServices(ctx)")
	if sanitize < 0 {
		t.Fatal("main.go 未找到 DeleteLegacyEmptyKeyCredentials 调用（清洗被删除：存量空明文凭据不再被清除）")
	}
	if background < 0 {
		t.Fatal("main.go 未找到 startBackgroundServices(ctx) 调用（装配结构已变，本门判据需同步更新）")
	}
	if sanitize > background {
		t.Fatalf("空凭据清洗调用（offset %d）在 startBackgroundServices（offset %d）之后：清洗必须早于后台任务与流量，避免与凭据状态机写路径交错", sanitize, background)
	}
}
