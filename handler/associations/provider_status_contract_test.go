package associations

import (
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// TestGetModelProviderStatus_SkipsNonOutcomeRows 锁「最近 10 次请求成败」的口径（状态机）：
// 只有 success 与 error 进窗口——processing（在途）、canceled（用户点停止/断开）、
// interrupted（进程先走了）三行既不是成也不是败，既不得占名额、更不得画成失败，
// 否则会在长流式请求期间、在用户自己撤走时给出「这家供应商在失败」的假结论。
//
// 夹具把**最新**一条造成 processing、倒数第二条造成 canceled、倒数第三条造成
// interrupted、最早一条造成 error，四条断言各自独立地钉住新口径：
//   - `data.0 = false`：列表按时间升序，最早那条 error 必须在场。老实现（不过滤
//     非结论行）会让三条非结论行顶掉它，data.0 变成 true。
//   - `data.9 = true`：最新一条是成功行。老实现里 data.9 就是那条 processing。
//   - false 条数恒为 1：只有那条 error。三条非结论行若被画成失败会多出 3 条。
//   - 长度恒为 10：13 行里只有 10 行是结论，非结论行不得占名额。
//
// 响应体 JSON 上断言（不读 Go 值）：这条契约就是「布尔数组 + 顺序 + 长度」，绕开
// 序列化层测不出形状。
func TestGetModelProviderStatus_SkipsNonOutcomeRows(t *testing.T) {
	testsupport.InitTestDB(t)

	provider := models.Provider{Name: "p-status-read"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	// created_at 显式递增：SQLite 的秒级时间戳在同一批插入里会并列，排序不稳。
	// 走变量赋值而非复合字面量：CreatedAt 由内嵌 gorm.Model 提升，字面量里写不进去。
	base := time.Now().Add(-time.Hour)
	newRow := func(i int, status string) {
		t.Helper()
		row := models.ChatLog{
			Name:          "m-status",
			ProviderName:  provider.Name,
			ProviderModel: "pm-status",
			Status:        status,
		}
		row.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if err := models.DB.Create(&row).Error; err != nil {
			t.Fatalf("create chat log #%d (%s): %v", i, status, err)
		}
	}
	newRow(0, models.ChatLogStatusError)
	for i := 1; i <= 9; i++ {
		newRow(i, models.ChatLogStatusSuccess)
	}
	newRow(10, models.ChatLogStatusCanceled)
	newRow(11, models.ChatLogStatusInterrupted)
	newRow(12, models.ChatLogStatusProcessing)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/model-providers/status?provider_id="+
		strconv.FormatUint(uint64(provider.ID), 10)+"&model_name=m-status&provider_model=pm-status", nil)

	GetModelProviderStatus(c)

	body := w.Body.String()
	if got := gjson.Get(body, "data.#").Int(); got != 10 {
		t.Fatalf("data 长度 = %d, want 10（13 行里非结论行必须被排除，不得占名额）:\n%s", got, body)
	}
	if gjson.Get(body, "data.0").Bool() {
		t.Errorf("data.0 = true, want false（最早那条是 error 行；老实现会让非结论行顶掉它）:\n%s", body)
	}
	if !gjson.Get(body, "data.9").Bool() {
		t.Errorf("data.9 = false, want true（最新那条是成功行，非结论行不得出现在列表里）:\n%s", body)
	}
	falseCount := 0
	for i := 0; i < 10; i++ {
		if !gjson.Get(body, fmt.Sprintf("data.%d", i)).Bool() {
			falseCount++
		}
	}
	if falseCount != 1 {
		t.Errorf("false 条数 = %d, want 1（只有那条 error 行；canceled/interrupted/processing 被画成失败会多出 3 条）:\n%s", falseCount, body)
	}
}
