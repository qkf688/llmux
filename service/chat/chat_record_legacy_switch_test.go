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

// pacedReader 给流式用例的读取加节拍：每 Read 只放行一小段并在返回前睡一小会儿，把
// 「首帧 → 收尾」的时间差做实。为什么必须加：bufio.Scanner 会把上游数据一次性读进缓冲、
// 之后逐行扫描全在内存里完成，「帧数多」并不能拉开首帧与末帧的时间差——实测 500 帧、甚至
// 每次 Read 前睡 25ms（首帧前只睡一次）都仍落在本机单调钟的同一 tick 内，chunkTime 恰为
// 0，TPS 断言会恒红。按字节限流才让每帧的抵达时间真正分开。
type pacedReader struct {
	src     io.Reader
	delay   time.Duration
	maxRead int
}

func (r pacedReader) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		p = p[:r.maxRead]
	}
	n, err := r.src.Read(p)
	if n > 0 {
		time.Sleep(r.delay)
	}
	return n, err
}

// openAIStreamSSEBody 造一条 OpenAI 风格 SSE 直通流：deltaFrames 个普通帧 + 末帧带 usage +
// [DONE]。
func openAIStreamSSEBody(deltaFrames int) string {
	var b strings.Builder
	for i := 0; i < deltaFrames; i++ {
		b.WriteString(`data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"x"}}]}` + "\n\n")
	}
	b.WriteString(`data: {"id":"1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}` + "\n\n")
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

// TestRecordLog_LegacyPerformanceSwitchesHaveNoEffect 锁「关掉两个性能开关」被删除后的
// 反向契约：**库里若还留着 true 的旧行，不得再影响任何行为**——usage 照解析入库、首包
// 时间照记（供应商响应时间样本照采）、TPS 照算。
//
// 为什么必须补这条「该行为继续保持不存在」的断言，而不是只删实现：旧键行是只有旧代码才
// 可能写入的留存数据、已无写入方；若只删实现不留用例，「这两个开关已无任何效力」这条契约
// 在用例层面就是空的——**同名**重新引入会立刻被本用例挡下（换成别的键名重新引入不在任何
// 用例的射程内，本用例不提供那个保证）。
//
// 覆盖两条链路：非流式与流式 SSE。processer 的计时/取数在两个分支各有一份代码，只测一边
// 时「另一边被加回开关门」仍会全绿（实测：只给 SSE 分支加回门，非流式单测照样绿）。
//
// seam：只走公共入口 RecordLog（真 processer ProcesserOpenAI + 真库），断言打在落库结果
// 的对外可观测值上（日志行、时间维度统计、真实模型统计、供应商统计），不伸手进内部函数。
func TestRecordLog_LegacyPerformanceSwitchesHaveNoEffect(t *testing.T) {
	cases := []struct {
		name        string
		stream      bool
		body        string
		readDelay   time.Duration
		expectTps   bool
		wantTokens  int64
		wantDaysAgo int
	}{
		{name: "非流式直通", stream: false, body: openAINonStreamUsageBody, wantTokens: 15, wantDaysAgo: 0},
		{name: "流式 SSE 直通", stream: true, body: openAIStreamSSEBody(20), readDelay: 2 * time.Millisecond, expectTps: true, wantTokens: 15, wantDaysAgo: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)

			for _, key := range legacyPerformanceSwitchKeys {
				upsertSettingValue(t, key, "true")
			}

			// 每例用独立的模型名 / 供应商名 / 日期：统计表按这几维分组，共用会让上一个
			// 用例的累计值串进本用例的断言。
			modelName := "m-" + tc.name
			providerName := "p-" + tc.name

			logRow := testutil.NewRow(t, models.DB, models.ChatLog{
				Name:          modelName,
				ProviderModel: "pm",
				ProviderName:  providerName,
				Status:        "success",
			})

			// 起点回拨 2 秒（每例再错开一天，落独立日期行）：首包时间必须是确定非零的
			// 可观测值。用 time.Now() 时 Windows 单调钟步进会让 time.Since 读数可能为 0，
			// 断言随机器漂移（红绿都不稳）。
			reqStart := time.Now().Add(-2*time.Second - time.Duration(tc.wantDaysAgo)*24*time.Hour)

			reader := io.Reader(strings.NewReader(tc.body))
			if tc.readDelay > 0 {
				reader = pacedReader{src: reader, delay: tc.readDelay, maxRead: 48}
			}

			RecordLog(context.Background(), RecordLogInput{
				ReqStart:      reqStart,
				Reader:        io.NopCloser(reader),
				Processer:     ProcesserOpenAI,
				Before:        Before{Model: modelName, Stream: tc.stream, raw: []byte(`{}`)},
				ProviderName:  providerName,
				RealModelName: modelName,
				LogID:         logRow.ID,
			})

			var row models.ChatLog
			if err := models.DB.First(&row, logRow.ID).Error; err != nil {
				t.Fatalf("load chat log: %v", err)
			}

			t.Run("usage 照解析入库", func(t *testing.T) {
				if row.Usage.TotalTokens != tc.wantTokens {
					t.Errorf("chat_logs.total_tokens = %d, want %d（旧开关行不得再让 usage 归零）",
						row.Usage.TotalTokens, tc.wantTokens)
				}
				if row.UsageSource == models.UsageSourceMissing {
					t.Errorf("UsageSource = %q, want 非 missing（直通链路本应标 passthrough，missing 表示取数被开关掐掉）",
						row.UsageSource)
				}

				var daily models.StatsDaily
				if err := models.DB.First(&daily, "date = ?", reqStart.Format("2006-01-02")).Error; err != nil {
					t.Fatalf("load stats daily: %v", err)
				}
				if daily.Tokens != tc.wantTokens {
					t.Errorf("stats_daily.tokens = %d, want %d", daily.Tokens, tc.wantTokens)
				}

				// 真实模型维度与供应商维度各自也吃同一份 usage：两列都断言，否则「开关
				// 掐掉取数」只在时间维度的累计值上留下证据，另两处静默为空也没人发现。
				var realModel models.StatsRealModelTotal
				if err := models.DB.First(&realModel, "name = ?", modelName).Error; err != nil {
					t.Fatalf("load stats real model: %v", err)
				}
				if realModel.TotalTokens != tc.wantTokens {
					t.Errorf("stats_real_model_total.total_tokens = %d, want %d",
						realModel.TotalTokens, tc.wantTokens)
				}

				var provider models.StatsProviderTotal
				if err := models.DB.First(&provider, "provider_name = ?", providerName).Error; err != nil {
					t.Fatalf("load provider stats: %v", err)
				}
				if provider.TotalTokens != tc.wantTokens {
					t.Errorf("stats_provider_total.total_tokens = %d, want %d",
						provider.TotalTokens, tc.wantTokens)
				}
			})

			t.Run("首包时间与 TPS 照记", func(t *testing.T) {
				if row.FirstChunkTime < time.Second {
					t.Errorf("chat_logs.first_chunk_time = %v, want ≥1s（旧开关行不得再让首包时间归零）",
						row.FirstChunkTime)
				}

				var provider models.StatsProviderTotal
				if err := models.DB.First(&provider, "provider_name = ?", providerName).Error; err != nil {
					t.Fatalf("load provider stats: %v", err)
				}
				if provider.ResponseTimeSamples == 0 || provider.AvgResponseTime == 0 {
					t.Errorf("provider stats = %d samples / %d cumulative rt, want 均非零（响应时间样本不得因旧开关行消失）",
						provider.ResponseTimeSamples, provider.AvgResponseTime)
				}

				if tc.expectTps && row.Tps <= 0 {
					t.Errorf("chat_logs.tps = %v, want > 0（流式链路必须算 TPS；非流式首帧即末帧，时间差为 0 故不断言）",
						row.Tps)
				}
			})
		})
	}
}
