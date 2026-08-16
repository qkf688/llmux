package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
)

func TestCleanupLogsIfNeeded_HardDeletesSoftDeletedBeyondRetention(t *testing.T) {
	initChatRecordTestDB(t)

	// 强制设置为 100，避免依赖默认值导致测试脆弱
	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyLogRetentionCount).
		Update("value", "100").Error; err != nil {
		t.Fatalf("set log retention count: %v", err)
	}

	logs := make([]models.ChatLog, 200)
	for i := range logs {
		logs[i] = models.ChatLog{
			Name:          "m1",
			ProviderModel: "pm1",
			ProviderName:  "p1",
			Status:        "success",
		}
		if err := models.DB.Create(&logs[i]).Error; err != nil {
			t.Fatalf("create log %d: %v", i, err)
		}
		if err := models.DB.Create(&models.ChatIO{
			LogID: logs[i].ID,
			Input: "in",
			OutputUnion: models.OutputUnion{
				OfString: "out",
			},
		}).Error; err != nil {
			t.Fatalf("create chat io %d: %v", i, err)
		}
	}

	// 模拟旧行为：先软删一部分日志（UI 仍只看到 100 条），但数据库仍保留软删记录
	idsToSoftDelete := make([]uint, 100)
	for i := 0; i < 100; i++ {
		idsToSoftDelete[i] = logs[i].ID
	}
	if err := models.DB.Where("id IN ?", idsToSoftDelete).Delete(&models.ChatLog{}).Error; err != nil {
		t.Fatalf("soft delete logs: %v", err)
	}

	var visible int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&visible).Error; err != nil {
		t.Fatalf("count visible logs: %v", err)
	}
	if visible != 100 {
		t.Fatalf("visible logs = %d, want 100", visible)
	}

	var total int64
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		t.Fatalf("count total logs: %v", err)
	}
	if total != 200 {
		t.Fatalf("total logs = %d, want 200", total)
	}

	cleanupLogsIfNeeded(context.Background())

	var afterTotal int64
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Count(&afterTotal).Error; err != nil {
		t.Fatalf("count total logs after cleanup: %v", err)
	}
	if afterTotal != 100 {
		t.Fatalf("total logs after cleanup = %d, want 100", afterTotal)
	}

	var softDeleted int64
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Where("deleted_at IS NOT NULL").Count(&softDeleted).Error; err != nil {
		t.Fatalf("count soft deleted logs: %v", err)
	}
	if softDeleted != 0 {
		t.Fatalf("soft deleted logs = %d, want 0", softDeleted)
	}

	var ioTotal int64
	if err := models.DB.Unscoped().Model(&models.ChatIO{}).Count(&ioTotal).Error; err != nil {
		t.Fatalf("count chat io: %v", err)
	}
	if ioTotal != 100 {
		t.Fatalf("chat io total = %d, want 100", ioTotal)
	}
}
