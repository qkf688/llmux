package chat

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

// TPS 分子口径回归：**必须用 completion_tokens，不得用 total_tokens**。
//
// 为什么必须有这条用例：全仓唯一既有覆盖（chat_record_legacy_switch_test.go 的
// `row.Tps > 0`）对分子写 total 还是 completion 都绿——口径回退时没有任何用例会红。
//
// 判据不走「跑两次比大小」：TPS 的分母 chunkTime 每次运行都不同，两次不可比。
// 改为拿被测自己交出的 ChunkTime 反算期望值——分子或分母任一被改动，等式即不成立。
const (
	tpsPromptTokens     = 1000 // 刻意 ≫ completion：分子若误用 total，值会大两个数量级
	tpsCompletionTokens = 5
)

// TestProcesser_TpsNumeratorIsCompletionTokens 覆盖三协议共用的 createProcesser 闭包。
// 表驱动两条式样（openai / anthropic）而非只测一条：口径写死在闭包里、三协议共用，
// 但「某条链路另开一份计算」是要防的退化，多一条式样即多一处哨兵。
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
			// pacedReader 让流跨多次 Read 到达：窗口必须显著非零，否则本用例的
			// 等式左边恒为 0、右边恒为 0，测不出任何东西（Windows 单调钟步进下
			// 快语句 elapsed 可为 0，故不能靠「跑一遍碰运气」）。
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
			if log.ChunkTime <= 0 {
				t.Fatalf("夹具退化：chunkTime=%v，窗口为 0 时两侧恒等、本用例不具判别力", log.ChunkTime)
			}

			want := float64(u.CompletionTokens) / log.ChunkTime.Seconds()
			if math.Abs(log.Tps-want) > want*1e-9 {
				t.Errorf("Tps = %v, want %v（= completion_tokens / chunkTime）；"+
					"差这么多说明分子不是 completion_tokens——用 total_tokens 会把 prompt 也算进"+
					"「每秒生成」，长上下文下数值被放大数千倍", log.Tps, want)
			}
		})
	}
}

// TestProcesser_Tps_NoCompletionTokensStaysZero 锁边界：上游只给 prompt/total
// （无 completion）时，TPS 必须是 0，**不得拿 total 兜底编造一个速度**。
//
// 这条是旧实现与新实现行为最分明的分界：旧分子（total）在此场景下会给出一个大数，
// 新分子（completion=0）给出 0。分母健康（非零窗口）也存在，故判据不靠「窗口为 0」兜底。
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
	if log.ChunkTime <= 0 {
		t.Fatalf("夹具退化：chunkTime=%v，本用例要验的是「窗口健康也不拿 total 兜底」", log.ChunkTime)
	}
	if log.Tps != 0 {
		t.Errorf("Tps = %v, want 0（completion_tokens 缺失时不得拿 total_tokens 兜底编造生成速度）", log.Tps)
	}
}
