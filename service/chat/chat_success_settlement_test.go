package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/channel"
)

// 关联级「这次算成功」的时点（Stage 44 补拍，2026-10-17）：**上游交出响应不是成功，正文
// 完整读完才是**。下面三条用例合起来是那条拍板的护栏，缺一条就有一半失效：
//  1. attempt 层不再动关联成功账——否则同一请求里「提交成功即清零」必然先于提交后失败的 +1，
//     稳定腰斩的家连败永远攒不到阈值（独立审查 2026-10-17 实测出的坑）；
//  2. 日志侧成功出口必须清零——否则连败一旦累计就再也不会归零，家被永久停用；
//  3. 提交后失败能把已累计的连败推过默认阈值并停用关联——这是拍板要的最终效果。
//
// 分开各自绿、合起来无效，正是这条拍板第一版翻车的原因，故三条都锁。

// seedAssociationAccounting 把关联的连败计数与权重设成可观测初值。
func seedAssociationAccounting(t *testing.T, id uint, consecutiveFailures, weight int) {
	t.Helper()
	if err := models.DB.Model(&models.ModelWithProvider{}).Where("id = ?", id).
		Updates(map[string]any{"consecutive_failures": consecutiveFailures, "weight": weight}).Error; err != nil {
		t.Fatalf("seed association accounting: %v", err)
	}
}

// readAssociationAccounting 读回关联的 (连败计数, 权重, status)。
func readAssociationAccounting(t *testing.T, id uint) (int, int, *bool) {
	t.Helper()
	var row models.ModelWithProvider
	if err := models.DB.First(&row, id).Error; err != nil {
		t.Fatalf("读回关联: %v", err)
	}
	return row.ConsecutiveFailures, row.Weight, row.Status
}

// TestBalanceChat_SuccessAttemptDoesNotResetAssociationAccounting 锁「attempt 层不再结关联
// 成功账」：上游交出响应（过完提交门）即算成功的旧口径已被本 Stage 取代。
//
// 变异自检验证——把 `adjustment.ResetConsecutiveFailures` / `ApplySuccessAdjustments` 加回
// `executeSingleProviderAttempt` 的成功出口：本用例红（计数被清零、权重被 +1）。
func TestBalanceChat_SuccessAttemptDoesNotResetAssociationAccounting(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	const goodBody = `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(goodBody))
	}))
	t.Cleanup(srv.Close)

	model := models.Model{Name: "settle-success-m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	mwp := setupGateFailoverProvider(t, cipher, model.ID, "ok", srv.URL, 1)
	seedAssociationAccounting(t, mwp.ID, 2, 50)

	if _, err := runBalanceOverPool(t, false, model.Name,
		[]byte(`{"model":"`+model.Name+`","messages":[{"role":"user","content":"hi"}]}`),
		[]models.ModelWithProvider{mwp}); err != nil {
		t.Fatalf("BalanceChat err = %v, want 成功", err)
	}

	gotFailures, gotWeight, _ := readAssociationAccounting(t, mwp.ID)
	if gotFailures != 2 || gotWeight != 50 {
		t.Fatalf("上游交出响应就把关联成功账结掉了：连败=%d 权重=%d, want 2/50（成功账的唯一时点是正文完整读完）", gotFailures, gotWeight)
	}
}

// TestRecordLog_SuccessExitResetsAssociationAccounting 锁成功账的唯一时点：日志侧成功出口
// （processer 读到流收尾）必须清零连败并按设置加权。它是上一条的另一半——少了它，连败一旦
// 累计就再也不会归零，被停用的关联再也回不来。
//
// 变异自检验证——删掉成功出口里的 `ResetConsecutiveFailures`：本用例红。
func TestRecordLog_SuccessExitResetsAssociationAccounting(t *testing.T) {
	initChatRecordTestDB(t)
	_, mwp := createAttributionFixtureProvider(t, "p-success-exit")
	seedAssociationAccounting(t, mwp.ID, 3, 50)

	procOK := func(context.Context, io.Reader, bool, time.Time) (*models.ChatLog, *models.OutputUnion, error) {
		return &models.ChatLog{Usage: models.Usage{TotalTokens: 1}}, &models.OutputUnion{OfString: "ok"}, nil
	}
	RecordLog(context.Background(), RecordLogInput{
		ReqStart:                   time.Now(),
		Reader:                     io.NopCloser(strings.NewReader("x")),
		Processer:                  procOK,
		LogID:                      0,
		ProviderName:               "p-success-exit",
		ServingModelWithProviderID: mwp.ID,
	})

	gotFailures, gotWeight, _ := readAssociationAccounting(t, mwp.ID)
	if gotFailures != 0 {
		t.Fatalf("日志侧成功出口没清零连败：%d, want 0", gotFailures)
	}
	if gotWeight <= 50 {
		t.Fatalf("日志侧成功出口没按设置加权：权重 %d, want > 50（`SettingKeyAutoSuccessIncrease` 默认开）", gotWeight)
	}
}

// TestRecordLog_PostCommitFailure_ReachesDisableThreshold 锁拍板要的最终效果：已累计到 2 的
// 连败被一次提交后失败推到默认阈值 3 → 关联被自动停用。它把「清零时点后移」与「提交后失败
// 计入连败」两半合起来看，正是第一版分开各自绿、合起来无效的那个缺口。
func TestRecordLog_PostCommitFailure_ReachesDisableThreshold(t *testing.T) {
	initChatRecordTestDB(t)
	providerRow, mwp := createAttributionFixtureProvider(t, "p-disable-threshold")
	enabled := true
	if err := models.DB.Model(&models.ModelWithProvider{}).Where("id = ?", mwp.ID).
		Updates(map[string]any{"consecutive_failures": 2, "status": &enabled}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:                   time.Now(),
		Reader:                     io.NopCloser(strings.NewReader("x")),
		Processer:                  failingProcesser(fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF)),
		LogID:                      0,
		ProviderName:               providerRow.Name,
		ServingModelWithProviderID: mwp.ID,
	})

	gotFailures, _, status := readAssociationAccounting(t, mwp.ID)
	if gotFailures != 3 {
		t.Fatalf("连败 = %d, want 3（提交后失败要能推过默认阈值）", gotFailures)
	}
	if status == nil || *status {
		t.Fatalf("连败达默认阈值 3 后关联未被自动停用：status=%v, want false", status)
	}
}
