package main

import (
	"os"
	"strings"
	"testing"
)

// Stage 16 审查回批 W1 机械门：存量头快照清洗的调用位置是隐式契约——必须在
// startBackgroundServices(ctx) 之前（清洗时刻无 chat_logs 并发写者）。若被挪到
// 后台任务/流量启动之后，清洗的「读入行 → Update」窗口会与 errors_only 策略的
// ClearRawFields 交错：行被读入后、Update 前恰好被清空时，清洗会把掩码值写回，
// 复活本应被清空的字段，破坏 errors_only 的隐私口径；删掉调用则存量明文不再被
// 脱敏。装配序其余部分的测试锁定归 Stage 18，本门只锁这一条安全面位置约束。
func TestGateStartupSanitizeRunsBeforeBackgroundServices(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读取 main.go: %v", err)
	}
	content := string(src)
	sanitize := strings.Index(content, "SanitizeLegacyLogSnapshots")
	background := strings.Index(content, "startBackgroundServices(ctx)")
	if sanitize < 0 {
		t.Fatal("main.go 未找到 SanitizeLegacyLogSnapshots 调用（清洗被删除：存量明文头快照不再被脱敏）")
	}
	if background < 0 {
		t.Fatal("main.go 未找到 startBackgroundServices(ctx) 调用（装配结构已变，本门判据需同步更新）")
	}
	if sanitize > background {
		t.Fatalf("存量清洗调用（offset %d）在 startBackgroundServices（offset %d）之后：清洗必须早于后台任务与流量，否则与 ClearRawFields 交错复活字段", sanitize, background)
	}
}
