package chat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// TestRecordLog_SuccessExitWritesSuccessStatus 锁状态机的成功出口：建行值是 processing
// （`executeSingleProviderAttempt` 的 logEntry），终态必须由成功出口**显式**落库。
// 夹具刻意造成 processing（而不是 success）——若成功出口不写终态，processer 返回的空
// Status 会被 `UpdateByID` 的 struct Updates 跳零值，行原样停在 processing，本用例变红。
func TestRecordLog_SuccessExitWritesSuccessStatus(t *testing.T) {
	initChatRecordTestDB(t)

	logRow := testutil.NewRow(t, models.DB, models.ChatLog{
		Name:          "m",
		ProviderModel: "pm",
		ProviderName:  "p",
		Status:        "processing",
	})

	runSuccessfulRecordLog(t, logRow.ID)

	var got models.ChatLog
	if err := models.DB.First(&got, logRow.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "success" {
		t.Fatalf("状态 = %q, want success（成功出口必须显式写终态，否则行停在建行值 processing）", got.Status)
	}
	if got.Error != "" {
		t.Fatalf("Error = %q, want 空（成功行不得带失败原因）", got.Error)
	}
}

// TestExecuteSingleProviderAttempt_RowStaysProcessingWhileUpstreamInFlight 锁状态机的建行值：
// 行在 `Client.Do` **之前**建（processing），上游阻塞期间库里读到的就是 processing，而不是
// 「已完成」。这条同时是「进程被 kill / 断电时行停在 processing」的可观测代理——那种窗口没法
// 在单测里杀进程，但「请求已发出、终态尚未落库」这一前提到此为止完全一致。
//
// 判据取「上游已收到请求」这个同步点：建行发生在 Client.Do 之前，故上游一收到请求，行必在库。
func TestExecuteSingleProviderAttempt_RowStaysProcessingWhileUpstreamInFlight(t *testing.T) {
	initChatRecordTestDB(t)

	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseUpstream := func() { releaseOnce.Do(func() { close(release) }) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-inflight","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(srv.Close)

	// 夹具在测试 goroutine 里建好（t.Fatalf 只能出现在这里），attempt 本身放进 goroutine。
	input := buildAttemptInputAgainstURL(t, "p-inflight", srv.URL, srv.Client(), context.Background(), smallReqRaw)
	done := make(chan singleProviderAttemptResult, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- executeSingleProviderAttempt(input, make(chan models.ChatLog, 8))
	}()

	// 无论断言在哪一步失败都要放行上游并等 attempt 收尾（defer 先于 t.Cleanup 跑，
	// 而库关闭在 Cleanup 里）：否则 srv.Close 等这只未完成的请求、attempt 又要写已被
	// 关掉的库，清理阶段互相卡死（本用例的失败分支曾实测挂住）。
	defer func() {
		releaseUpstream()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
		}
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("上游未收到请求（建行在 Client.Do 之前，上游收到即行已落库）")
	}

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-inflight").First(&row).Error; err != nil {
		t.Fatalf("reload in-flight log: %v", err)
	}
	if row.Status != "processing" {
		t.Fatalf("在途行状态 = %q, want processing（终态归成功出口，此刻还没到）", row.Status)
	}

	releaseUpstream()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("attempt 未在放行后结束")
	}
	result := <-done
	if !result.Success {
		t.Fatalf("放行后 attempt 应走成功分支: %+v", result)
	}
	if result.Response != nil && result.Response.Body != nil {
		_ = result.Response.Body.Close()
	}
}
