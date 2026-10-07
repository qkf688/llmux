package chat

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// legacyPerformanceSwitchKeys 是本用例要模拟的历史行键名，**刻意写裸字符串**：
// 这两个开关连同 models.SettingKey* 常量一起被删除，引用常量会让用例编译不过；
// 而本用例要锁的恰恰是「键已不在 schema 里、库里仍留着 true 旧行」这一状态。
var legacyPerformanceSwitchKeys = []string{
	"disable_performance_tracking",
	"disable_token_counting",
}

// upsertSettingValue 按裸键写 settings 行：models.Init 会按 schema 播默认行，
// 键从 schema 删除后不再播——故「先找后建」，两种形态（有行 / 无行）都要能造。
func upsertSettingValue(t *testing.T, key, value string) {
	t.Helper()

	var row models.Setting
	err := models.DB.Where("key = ?", key).First(&row).Error
	switch {
	case err == nil:
		if err := models.DB.Model(&models.Setting{}).
			Where("key = ?", key).Update("value", value).Error; err != nil {
			t.Fatalf("update setting %s: %v", key, err)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err := models.DB.Create(&models.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatalf("create setting %s: %v", key, err)
		}
	default:
		t.Fatalf("load setting %s: %v", key, err)
	}
}

// openAINonStreamUsageBody 是直通链路（不做协议转换）下 processer 真正要读的那条
// 上游响应：非流式 = 单行 JSON，usage 与正文同体。
const openAINonStreamUsageBody = `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`

// TestRecordLog_LegacyPerformanceSwitchesHaveNoEffect 锁「关掉两个性能开关」被删除后的
// 反向契约：**库里若还留着 true 的旧行，不得再影响任何行为**——usage 照解析入库、
// 首包时间照记（供应商响应时间样本照采）。
//
// 为什么必须补这条「该行为继续保持不存在」的断言，而不是只删实现：旧键的行是只有旧
// 代码才可能写入的留存数据、已无写入方，删实现不删用例时「这两个开关已无任何效力」
// 这条契约在用例层面是空的——将来有人换个名字把开关加回来，全量测试照样全绿。
//
// seam：只走公共入口 RecordLog（真 processer ProcesserOpenAI + 真库），断言打在
// 落库结果的对外可观测值上（日志行、时间维度统计、供应商统计），不伸手进内部函数。
func TestRecordLog_LegacyPerformanceSwitchesHaveNoEffect(t *testing.T) {
	initChatRecordTestDB(t)

	for _, key := range legacyPerformanceSwitchKeys {
		upsertSettingValue(t, key, "true")
	}

	logRow := testutil.NewRow(t, models.DB, models.ChatLog{
		Name:          "m1",
		ProviderModel: "pm1",
		ProviderName:  "p1",
		Status:        "success",
	})
	logID := logRow.ID

	// 起点回拨 2 秒：首包时间必须是确定非零的可观测值。用 time.Now() 时 Windows
	// 单调钟步进会让 time.Since 读数可能为 0，断言随机器漂移（红绿都不稳）。
	reqStart := time.Now().Add(-2 * time.Second)

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:      reqStart,
		Reader:        io.NopCloser(strings.NewReader(openAINonStreamUsageBody)),
		Processer:     ProcesserOpenAI,
		Before:        Before{Model: "m1", Stream: false, raw: []byte(`{}`)},
		ProviderName:  "p1",
		RealModelName: "m1",
		LogID:         logID,
	})

	t.Run("usage 照解析入库", func(t *testing.T) {
		var row models.ChatLog
		if err := models.DB.First(&row, logID).Error; err != nil {
			t.Fatalf("load chat log: %v", err)
		}
		if row.Usage.TotalTokens != 15 {
			t.Errorf("chat_logs.total_tokens = %d, want 15（旧开关行不得再让 usage 归零）",
				row.Usage.TotalTokens)
		}
		if row.UsageSource == models.UsageSourceMissing {
			t.Errorf("UsageSource = %q, want 非 missing（直通链路本应标 passthrough，missing 表示取数被开关掐掉）",
				row.UsageSource)
		}

		var daily models.StatsDaily
		if err := models.DB.First(&daily, "date = ?", reqStart.Format("2006-01-02")).Error; err != nil {
			t.Fatalf("load stats daily: %v", err)
		}
		if daily.Tokens != 15 {
			t.Errorf("stats_daily.tokens = %d, want 15", daily.Tokens)
		}
	})

	t.Run("首包时间照记", func(t *testing.T) {
		var row models.ChatLog
		if err := models.DB.First(&row, logID).Error; err != nil {
			t.Fatalf("load chat log: %v", err)
		}
		if row.FirstChunkTime < time.Second {
			t.Errorf("chat_logs.first_chunk_time = %v, want ≥1s（旧开关行不得再让首包时间归零）",
				row.FirstChunkTime)
		}

		var provider models.StatsProviderTotal
		if err := models.DB.First(&provider, "provider_name = ?", "p1").Error; err != nil {
			t.Fatalf("load provider stats: %v", err)
		}
		if provider.ResponseTimeSamples == 0 || provider.AvgResponseTime == 0 {
			t.Errorf("provider stats = %d samples / %d cumulative rt, want 均非零（响应时间样本不得因旧开关行消失）",
				provider.ResponseTimeSamples, provider.AvgResponseTime)
		}
	})
}
