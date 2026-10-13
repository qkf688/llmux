package chat

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

// TPS 口径回归：**分子 = completion_tokens，分母 = 整段耗时（elapsed）**。
//
// 为什么必须有这条用例：全仓另一处覆盖（chat_record_legacy_switch_test.go 的
// `row.Tps > 0`）对分子写 total 还是 completion、分母写 chunkTime 还是 elapsed
// 都绿——口径回退时没有任何用例会红。
//
// 判据不靠「跑两次比大小」（绝对耗时每次运行都不同、两次不可比），而是拿被测自己
// 交出的 FirstChunkTime + ChunkTime 反算期望值——被测内部正是 `chunkTime :=
// elapsed - firstChunkTime`，故 elapsed 恒等于两者之和。任一被改动，等式即不成立。
//
// **firstChunkTime 被刻意做成显著非零**（pacedReader 分次投喂）：若分母误用
// chunkTime，两者差一个 firstChunkTime，判据立刻能检出——这正是本用例要区分的两条口径。
const (
	tpsPromptTokens     = 1000 // 刻意 ≫ completion：分子若误用 total，值会大两个数量级
	tpsCompletionTokens = 5
)

// TestProcesser_TpsNumeratorIsCompletionTokens 覆盖三协议共用的 createProcesser 闭包。
// 表驱动两条式样（openai / anthropic）而非只测一条：「某条链路另开一份计算」是
// 要防的退化，多一条式样即多一处哨兵。
func TestProcesser_TpsNumeratorIsCompletionTokens(t *testing.T) {
	cases := []struct {
		name      string
		processer Processer
		body      string
	}{
		{
			name:      "openai 流式",
			processer: ProcesserOpenAI,
			body: `data: {"id":"1","choices":[{"delta":{"content":"x"}}]}` + "\n\n" +
				`data: {"id":"1","choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":5,"total_tokens":1005}}` + "\n\n" +
				"data: [DONE]\n\n",
		},
		{
			name:      "anthropic 流式",
			processer: ProcesserAnthropic,
			body: "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":1000,\"output_tokens\":5}}\n\n" +
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// pacedReader 让流跨多次 Read 到达：firstChunkTime（首帧到达前的耗时）与
			// chunkTime（首帧到读完）都必须显著非零，否则两条分母口径数值相同、
			// 判据不具判别力（Windows 单调钟步进下快语句 elapsed 可为 0）。
			reader := pacedReader{
				src:     strings.NewReader(tc.body),
				delay:   2 * time.Millisecond,
				maxRead: 48,
			}

			log, _, err := tc.processer(context.Background(), reader, true, time.Now())
			if err != nil {
				t.Fatalf("processer: %v", err)
			}

			u := log.Usage
			// 夹具自检：样本必须满足「total 与 completion 差一个量级」，否则分子口径
			// 写错也测不出差异（断言恒真的假绿）。
			if u.TotalTokens <= u.CompletionTokens*10 {
				t.Fatalf("夹具退化：total=%d 未显著大于 completion=%d，分子口径写错也测不出差异",
					u.TotalTokens, u.CompletionTokens)
			}
			// 夹具自检：首帧前耗时必须显著，否则「整段耗时」与「首帧后耗时」不可区分
			// （用 chunkTime 做分母的实现会照样通过，判据失去对分母的覆盖）。
			if log.FirstChunkTime <= 0 {
				t.Fatalf("夹具退化：FirstChunkTime=%v，两种分母口径在此样本上等值、无法区分",
					log.FirstChunkTime)
			}

			// 被测内部：chunkTime = elapsed - firstChunkTime ⇒ elapsed = 两者之和。
			elapsed := log.FirstChunkTime + log.ChunkTime
			want := float64(u.CompletionTokens) / elapsed.Seconds()
			if math.Abs(log.Tps-want) > want*1e-9 {
				t.Errorf("Tps = %v, want %v（= completion_tokens / 整段耗时 %v）；"+
					"分子用 total_tokens 会放大数值，分母用 chunkTime（首帧后那段）会把"+
					"「上游成批下发」的请求算爆表", log.Tps, want, elapsed)
			}
		})
	}
}

// TestProcesser_Tps_NoCompletionTokensStaysZero 锁边界：上游只给 prompt/total
// （无 completion）时，TPS 必须是 0，**不得拿 total 兜底编造一个速度**。
//
// 这条是「分子」两版行为最分明的分界：旧分子（total）在此场景下会给出一个大数，
// 新分子（completion=0）给出 0。分母健康（耗时为正）也存在，判据不靠「耗时为 0」兜底。
func TestProcesser_Tps_NoCompletionTokensStaysZero(t *testing.T) {
	body := `data: {"id":"1","choices":[{"delta":{"content":"x"}}]}` + "\n\n" +
		`data: {"id":"1","choices":[],"usage":{"prompt_tokens":1000,"total_tokens":1000}}` + "\n\n" +
		"data: [DONE]\n\n"

	reader := pacedReader{src: strings.NewReader(body), delay: 2 * time.Millisecond, maxRead: 48}
	log, _, err := ProcesserOpenAI(context.Background(), reader, true, time.Now())
	if err != nil {
		t.Fatalf("ProcesserOpenAI: %v", err)
	}

	if log.Usage.TotalTokens == 0 {
		t.Fatalf("夹具退化：total_tokens 为 0，无法区分「按 completion 算得 0」与「根本没取到 usage」")
	}
	if log.Tps != 0 {
		t.Errorf("Tps = %v, want 0（completion_tokens 缺失时不得拿 total_tokens 兜底编造速度）", log.Tps)
	}
}
