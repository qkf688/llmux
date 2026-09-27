package common

import (
	"testing"
)

// TestModelsListReadLimit 钉住 Models() 读体上限的定档理由（Stage 27）：模型清单是
// 上游响应里最小的对象之一（每模型 ~100-200B：id/object/created/owned_by 四键），
// 现实供应商从几个到几千个模型（openrouter 级聚合商 ~500 个 ≈ 100 KiB），10 MiB 档
// 足以容纳数百倍余量；超限只可能是上游异常/被篡改，按解码失败处置。
// 本测试防的是「常量被静默改小导致大清单供应商同步失败」或「被改到无意义的天文数字」。
func TestModelsListReadLimit(t *testing.T) {
	if ModelsListReadLimitBytes < 1<<20 {
		t.Fatalf("ModelsListReadLimitBytes = %d, 不应小于 1 MiB（现实聚合商清单已达 ~100 KiB 量级）", ModelsListReadLimitBytes)
	}
	if ModelsListReadLimitBytes > 64<<20 {
		t.Fatalf("ModelsListReadLimitBytes = %d, 超过 64 MiB 已失去「挡异常超大响应」的意义", ModelsListReadLimitBytes)
	}
}
