package associations

import (
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/tidwall/gjson"
)

// 空上游名写入闸（Stage 42 方案 A / 关联可保存空上游模型名）。
//
// 为什么闸必须落在后端：Create 路径的空串只被前端 `handleCreate` 的 toast 拦了一道，
// 直接调接口与 `handler/importexport` 的整批 `tx.Create` 都绕得开。
// 为什么判 TrimSpace：Update 走 struct Updates，空串被 GORM 当零值跳过（原值保留），
// 只有**全空格**能被写进去——不判 TrimSpace 就漏掉这条真实可达路径。
// 两条路径的语义差异：Create 的 provider_model 是定义性字段（空必被上游 400），
// Update 缺省=不改（部分更新语义），所以只有「传了但为空」才拒。

func TestCreateModelProvider_RejectsBlankProviderModel(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	cases := []struct {
		name string
		body string
	}{
		{"空串", `{"model_id":1,"provider_id":1,"provider_model":"","weight":10,"priority":10}`},
		{"全空格", `{"model_id":1,"provider_id":1,"provider_model":"   ","weight":10,"priority":10}`},
		{"键缺失", `{"model_id":1,"provider_id":1,"weight":10,"priority":10}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postAssocRaw(t, true, 0, tc.body)
			assertGateRejected(t, resp, "provider_model")
		})
	}

	// 被拒不落库：三条全部拒绝后列表仍为空。
	if n := gjson.Get(listAssocsViaHandler(t, 1), "data.#").Int(); n != 0 {
		t.Fatalf("rejected creates persisted: list len = %d, want 0", n)
	}
}

func TestUpdateModelProvider_RejectsBlankProviderModel(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t) // ProviderModel: "pm"

	// 原值非空 + 新值全空格：非零值，会被 struct Updates 真正写入 → 必须被闸拦住。
	resp := postAssocRaw(t, false, mp.ID,
		`{"model_id":1,"provider_id":1,"provider_model":"   ","with_header":false,"weight":10,"priority":10}`)
	assertGateRejected(t, resp, "provider_model")
	if got := reloadAssoc(t, mp.ID); got.ProviderModel != "pm" {
		t.Fatalf("rejected update mutated ProviderModel: %q, want pm", got.ProviderModel)
	}

	// 缺省=不改（部分更新语义）：省略该键时原值必须保留，且请求成功。
	resp = postAssocRaw(t, false, mp.ID,
		`{"model_id":1,"provider_id":1,"with_header":false,"weight":20,"priority":20}`)
	if code := gjson.Get(resp, "code").Int(); code != 200 {
		t.Fatalf("omitted provider_model rejected: envelope code = %d, want 200, body=%s", code, resp)
	}
	got := reloadAssoc(t, mp.ID)
	if got.ProviderModel != "pm" {
		t.Fatalf("omitted provider_model cleared value: %q, want pm", got.ProviderModel)
	}
	if got.Weight != 20 {
		t.Fatalf("Weight = %d, want 20 (partial update still applied)", got.Weight)
	}
}
