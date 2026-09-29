package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// Stage 17 收口（AC-7/AC-8）：chat_logs 全部写库点（失败出口回写 + 建行）不得
// 陪葬请求 ctx——最需要留痕的失败（超时、客户端断连）恰恰发生在请求 ctx 已死
// 的时刻，陪葬会让行停在建行占位的 success（审计与指标双双失真）或干脆无行。
// 判据：ctx 已取消时写库仍成功（context.WithoutCancel + 独立短超时派生）。

func TestUpdateChatLogByID_DeadCtxStillWrites(t *testing.T) {
	initChatRecordTestDB(t)

	log := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m-detach", Status: "success", Style: "openai"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 请求已死：超时/断连场景的等价形态

	updateChatLogByID(ctx, log.ID,
		models.ChatLog{Status: "error", Error: "upstream timeout"}, "failed to update log status")

	var row models.ChatLog
	if err := models.DB.First(&row, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.Status != "error" || row.Error != "upstream timeout" {
		t.Fatalf("cancelled ctx 下回写丢失: row = %q/%q, want error/upstream timeout", row.Status, row.Error)
	}
}

func TestSavePreProviderFailureLog_DeadCtxStillCreates(t *testing.T) {
	initChatRecordTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reqMeta := models.ReqMeta{RequestID: "gw:detach-test"}
	savePreProviderFailureLog(ctx, reqMeta, string(consts.StyleOpenAI), preProviderFailureLog{
		RequestedModel: "m-pre-fail",
		Error:          "model not found",
	})

	var row models.ChatLog
	if err := models.DB.Where("name = ?", "m-pre-fail").First(&row).Error; err != nil {
		t.Fatalf("cancelled ctx 下失败行未落库（审计缺口）: %v", err)
	}
	if row.Status != "error" || row.Error != "model not found" {
		t.Fatalf("失败行内容 = %q/%q, want error/model not found", row.Status, row.Error)
	}
	if row.RequestID != "gw:detach-test" {
		t.Fatalf("失败行 request_id = %q, want gw:detach-test（ReqMeta 快照随建行定格）", row.RequestID)
	}
}

// disable_all_logs 时建行返回 logID=0：本用例锁「logID=0 的更新不误伤其他行」。
// 短路分支本身是防御性实现细节（不短路时 GORM Where id=0 同样零行命中），
// 不可由本用例区分——如实标注锁定口径，勿把「删掉短路仍绿」误读成测试坏了。
func TestUpdateChatLogByID_LogIDZeroShortCircuits(t *testing.T) {
	initChatRecordTestDB(t)

	log := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m-bystander", Status: "success", Style: "openai"})

	updateChatLogByID(context.Background(), 0,
		models.ChatLog{Status: "error", Error: "should touch nothing"}, "failed to update log status")

	var row models.ChatLog
	if err := models.DB.First(&row, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.Status != "success" {
		t.Fatalf("旁观行被误改: %q", row.Status)
	}
}
