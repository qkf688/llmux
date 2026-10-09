package main

import (
	"os"
	"strings"
	"testing"
)

// 中断态启动兜底的机械门（状态机分片 #3）：MarkUnfinishedLogsInterrupted 与另两处
// 启动清洗同属「接任何流量之前必须跑完」的装配，位置是隐式契约——挪到
// startBackgroundServices(ctx) 之后，清洗时刻就可能已有本进程的在途请求，其行同样
// 停在建行值 processing，会被误判成「上次进程非正常退出」而写成 interrupted，把正常
// 在途请求伪装成中断记录。删掉调用则残留行永远停在 processing，前端按 #4 消费时
// 会一直显示「进行中」。判据结构同 main_startup_sanitize_gate_test.go。
func TestGateStartupInterruptedLogFallbackRunsBeforeBackgroundServices(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读取 main.go: %v", err)
	}
	content := string(src)
	fallback := strings.Index(content, "MarkUnfinishedLogsInterrupted")
	background := strings.Index(content, "startBackgroundServices(ctx)")
	if fallback < 0 {
		t.Fatal("main.go 未找到 MarkUnfinishedLogsInterrupted 调用（兜底被删除：上次进程非正常退出的残留行永远停在 processing）")
	}
	if background < 0 {
		t.Fatal("main.go 未找到 startBackgroundServices(ctx) 调用（装配结构已变，本门判据需同步更新）")
	}
	if fallback > background {
		t.Fatalf("中断兜底调用（offset %d）在 startBackgroundServices（offset %d）之后：必须早于后台任务与流量，否则本进程在途请求会被误判成上次进程的中断残留", fallback, background)
	}
}
