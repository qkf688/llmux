package metrics

import (
	"testing"

	"github.com/tidwall/gjson"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// realModelCountsJSON 调 RealModelCounts 并返回响应体原始 JSON。
func realModelCountsJSON(t *testing.T) string {
	t.Helper()
	c, w := testsupport.NewTestContext("GET", "/metrics/real-model-counts")
	RealModelCounts(c)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// TestRealModelCounts_TokenFieldsInResponseBody 钉住响应体的键名与取值。
//
// 断言打在 body 上而非 Go 结构体上：前端按 JSON 键名消费，键名写错时
// 反序列化到结构体仍然「通过」，只有打 body 才测得出来这类断层。
//
// 用例覆盖两类行：带 token 统计的（9 月起）与不带的（更早的历史调用）。
// 后者是本表与 token 统计起点的真实关系——早于统计落地的调用只有 calls，
// token 三列恒 0，且必须以「键存在且为 0」出现，不能被 omitempty 抹掉：
// 键缺失会让前端无法区分「没统计过」与「统计了但是 0」。
func TestRealModelCounts_TokenFieldsInResponseBody(t *testing.T) {
	testsupport.InitTestDB(t)

	rows := []models.StatsRealModelTotal{
		{Name: "with-tokens", Calls: 10, PromptTokens: 300, CompletionTokens: 120, TotalTokens: 420},
		{Name: "calls-only", Calls: 5},
	}
	for _, row := range rows {
		if err := models.DB.Create(&row).Error; err != nil {
			t.Fatalf("create stats real model total %s: %v", row.Name, err)
		}
	}

	first := gjson.Get(realModelCountsJSON(t), "data.0")
	if first.Get("model").String() != "with-tokens" {
		t.Fatalf("data.0 = %s, want with-tokens（按 calls 降序）", first.Raw)
	}
	for path, want := range map[string]int64{
		"calls":             10,
		"prompt_tokens":     300,
		"completion_tokens": 120,
		"total_tokens":      420,
	} {
		got := first.Get(path)
		if !got.Exists() {
			t.Fatalf("data.0.%s missing, want %d（键必须存在）", path, want)
		}
		if got.Int() != want {
			t.Fatalf("data.0.%s = %d, want %d", path, got.Int(), want)
		}
	}

	second := gjson.Get(realModelCountsJSON(t), "data.1")
	for _, path := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		if !second.Get(path).Exists() {
			t.Fatalf("data.1.%s missing on a calls-only row, want explicit 0", path)
		}
	}
	if second.Get("total_tokens").Int() != 0 {
		t.Fatalf("data.1.total_tokens = %d, want 0", second.Get("total_tokens").Int())
	}
}
