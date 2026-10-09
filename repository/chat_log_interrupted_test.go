package repository

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
)

// 启动兜底契约（状态机分片 #3）：进程非正常退出（被 kill / 断电）时，chat_logs 行会
// 永久停在建行值 processing——本次启动把这些行改写为 interrupted。
//
// 三条断言各自钉一个维度，缺一条都测不出真退化：
//   - 只改 processing 行：success / error / canceled / interrupted 终态行零改写
//     （判据放宽成「所有非终态行」或「所有行」都会在这里露馅）。
//   - 幂等：二次执行零改写——已改写的行不再是 processing，不该被重复计数。
//   - 幂等的可观测形态是**返回值**：行数不变之外还要求第二次返回 0，否则调用方
//     每次启动都会打一条「中断 N 行」的误导日志。
//
// 另外锁住「只改状态、不删行、不动其他字段」：这是 AGENTS §0 判据下「维护真实不变量」
// 的最小动作，行若被删就变成了数据丢失。
func TestChatLogRepo_MarkUnfinishedLogsInterrupted(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	seed := []struct {
		name   string
		status string
	}{
		{"m-processing", models.ChatLogStatusProcessing},
		{"m-success", models.ChatLogStatusSuccess},
		{"m-error", models.ChatLogStatusError},
		{"m-canceled", models.ChatLogStatusCanceled},
		{"m-interrupted", models.ChatLogStatusInterrupted},
	}
	for _, s := range seed {
		row := models.ChatLog{Name: s.name, ProviderName: "p", Status: s.status}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed %s: %v", s.name, err)
		}
	}

	updated, err := repo.MarkUnfinishedLogsInterrupted(ctx)
	if err != nil {
		t.Fatalf("MarkUnfinishedLogsInterrupted: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1（只改停在 processing 的行）", updated)
	}

	var processing models.ChatLog
	if err := db.Where("name = ?", "m-processing").First(&processing).Error; err != nil {
		t.Fatalf("reload processing row: %v", err)
	}
	if processing.Status != models.ChatLogStatusInterrupted {
		t.Fatalf("processing 行状态 = %q, want interrupted", processing.Status)
	}
	if processing.ProviderName != "p" {
		t.Fatalf("ProviderName = %q, want p（兜底只改状态，其他字段不得被覆盖）", processing.ProviderName)
	}

	// 终态行逐字相等：只断言「没变成 interrupted」会被「被改成别的值」蒙混过去。
	for _, name := range []string{"m-success", "m-error", "m-canceled", "m-interrupted"} {
		want := map[string]string{
			"m-success":     models.ChatLogStatusSuccess,
			"m-error":       models.ChatLogStatusError,
			"m-canceled":    models.ChatLogStatusCanceled,
			"m-interrupted": models.ChatLogStatusInterrupted,
		}[name]
		var row models.ChatLog
		if err := db.Where("name = ?", name).First(&row).Error; err != nil {
			t.Fatalf("reload %s: %v", name, err)
		}
		if row.Status != want {
			t.Fatalf("%s 状态 = %q, want %q（终态行必须零改写）", name, row.Status, want)
		}
	}

	// 行数不变：清洗只改状态、不删行。
	var total int64
	if err := db.Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != int64(len(seed)) {
		t.Fatalf("行数 = %d, want %d（兜底只改状态、绝不删行）", total, len(seed))
	}

	updatedAgain, err := repo.MarkUnfinishedLogsInterrupted(ctx)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if updatedAgain != 0 {
		t.Fatalf("二次执行 updated = %d, want 0（幂等：无 processing 行可改）", updatedAgain)
	}
}

// TestChatLogRepo_MarkUnfinishedLogsInterrupted_EmptyTable 边界：没有残留行时不得报错、
// 不得返回负数或非零——它是每次启动都跑的常驻清洗，空表是常态而非异常。
func TestChatLogRepo_MarkUnfinishedLogsInterrupted_EmptyTable(t *testing.T) {
	ctx := context.Background()
	db := newLogTestDB(t)
	repo := NewChatLogRepo(db)

	updated, err := repo.MarkUnfinishedLogsInterrupted(ctx)
	if err != nil {
		t.Fatalf("空表执行: %v", err)
	}
	if updated != 0 {
		t.Fatalf("updated = %d, want 0", updated)
	}
}
