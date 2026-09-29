package healthcheck

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// 本文件锁 Stage 36 的核心口径：一次检测只有两种东西——上游给的结论，与没走到上游。
// 后者记成「未完成」（not_measured），不计判停、不衰减、不打断连续失败。

func seedModelAndAssociation(t *testing.T, providerID uint) models.ModelWithProvider {
	t.Helper()

	model := models.Model{Name: "gpt-4.1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	enabled := true
	mp := models.ModelWithProvider{
		ModelID: model.ID, ProviderID: providerID, ProviderModel: "gpt-4.1",
		Weight: 10, Priority: 10, Status: &enabled,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	return mp
}

func latestHealthCheckLog(t *testing.T, mpID uint) models.HealthCheckLog {
	t.Helper()

	var got models.HealthCheckLog
	if err := models.DB.Where("model_provider_id = ?", mpID).Order("id DESC").First(&got).Error; err != nil {
		t.Fatalf("load latest health check log: %v", err)
	}
	return got
}

func assertAssociationStillEnabled(t *testing.T, mpID uint) {
	t.Helper()

	got, err := repository.Default().ModelWithProvider.Get(context.Background(), mpID)
	if err != nil {
		t.Fatalf("reload association: %v", err)
	}
	if got.Status == nil || !*got.Status {
		t.Fatalf("status = %v, want enabled（本机没测成不得停用关联）", got.Status)
	}
}

// TestCheckOneWithBatch_LocalReasons_WriteNotMeasured 本机原因没测成必须记「未完成」：
// 记成失败会让数据库抖一阵子就攒出连续失败，恢复后按真阈值把可用关联停用。
func TestCheckOneWithBatch_LocalReasons_WriteNotMeasured(t *testing.T) {
	cases := []struct {
		name  string
		seed  func(t *testing.T, providerID uint)
		after func(t *testing.T, hits *atomic.Int64)
	}{
		{
			name: "无启用的主协议端点",
			seed: func(t *testing.T, providerID uint) {
				// 只有 anthropic 端点：openai 主协议端点缺失
				seedEndpoint(t, providerID, "anthropic", "https://unused.example")
			},
		},
		{
			name: "没有任何分组",
			seed: func(t *testing.T, providerID uint) {
				seedEndpoint(t, providerID, "openai", "https://unused.example")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, mgr := newTestChecker(t)
			defer mgr.Shutdown(context.Background())

			provider := models.Provider{Name: "p-local", Type: "openai", Config: `{}`}
			if err := models.DB.Create(&provider).Error; err != nil {
				t.Fatalf("create provider: %v", err)
			}
			tc.seed(t, provider.ID)
			mp := seedModelAndAssociation(t, provider.ID)

			h.checkOneWithBatch(context.Background(), &mp, "b-local")

			got := latestHealthCheckLog(t, mp.ID)
			if got.Status != models.HealthCheckStatusNotMeasured {
				t.Fatalf("log status = %q (%s), want %q（本机原因不是上游的结论）",
					got.Status, got.Error, models.HealthCheckStatusNotMeasured)
			}
			if got.Error == "" {
				t.Error("未完成记录必须留下原因（health_check_logs.error 是排查入口）")
			}
			assertAssociationStillEnabled(t, mp.ID)
		})
	}
}

// TestCheckOneWithBatch_UpstreamFailure_WritesError 上游给了坏结论仍记「失败」：
// 分类不能把真失败一起放过，否则判停永远不触发。
func TestCheckOneWithBatch_UpstreamFailure_WritesError(t *testing.T) {
	h, mgr := newTestChecker(t)
	defer mgr.Shutdown(context.Background())
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	// 假上游不认这把 key → 401，属「请求发出去、拿到上游坏结论」
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-up", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	seedInlineGroup(t, cipher, provider.ID, "g1", "sk-rejected")
	mp := seedModelAndAssociation(t, provider.ID)

	h.checkOneWithBatch(context.Background(), &mp, "b-up")

	if hits.Load() == 0 {
		t.Fatal("假上游没收到请求，用例没打到上游失败分支")
	}
	got := latestHealthCheckLog(t, mp.ID)
	if got.Status != models.HealthCheckStatusError {
		t.Fatalf("log status = %q (%s), want %q", got.Status, got.Error, models.HealthCheckStatusError)
	}
}

// TestHandleCheckResult_NotMeasuredSkipsDecayAndDisable 「未完成」不衰减、不判停；
// 后半段用同一前置条件下的上游失败做反向断言，防止整段退化成 no-op 的假绿。
func TestHandleCheckResult_NotMeasuredSkipsDecayAndDisable(t *testing.T) {
	initHealthCheckTestDB(t)
	setSetting(t, models.SettingKeyHealthCheckFailureThreshold, "1")

	ctx := context.Background()
	enabled := true
	mp := models.ModelWithProvider{
		ModelID: 1, ProviderID: 1, ProviderModel: "pm",
		Weight: 10, Priority: 10, Status: &enabled,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	if err := models.DB.Create(&models.HealthCheckLog{
		ModelProviderID: mp.ID, Status: models.HealthCheckStatusError,
	}).Error; err != nil {
		t.Fatalf("create health check log: %v", err)
	}

	decayCalls := 0
	SetAdjustmentHooks(AdjustmentHooks{
		ShouldCountHealthCheckFailure: func(context.Context) bool { return true },
		ApplyWeightDecay:              func(context.Context, uint, string, string) { decayCalls++ },
		ApplyPriorityDecay:            func(context.Context, uint, string, string) { decayCalls++ },
	})
	t.Cleanup(func() { SetAdjustmentHooks(AdjustmentHooks{}) })

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", models.HealthCheckStatusNotMeasured)

	if decayCalls != 0 {
		t.Errorf("未完成触发了 %d 次权重/优先级衰减，want 0（不把本机问题记在供应商头上）", decayCalls)
	}
	assertAssociationStillEnabled(t, mp.ID)

	// 反向：同样的前置条件下，上游失败必须衰减 + 达阈值停用
	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", models.HealthCheckStatusError)

	if decayCalls != 2 {
		t.Errorf("上游失败触发 %d 次衰减，want 2（分类不能让失败分支整体失效）", decayCalls)
	}
	got, err := repository.Default().ModelWithProvider.Get(ctx, mp.ID)
	if err != nil {
		t.Fatalf("reload association: %v", err)
	}
	if got.Status == nil || *got.Status {
		t.Errorf("status = %v, want disabled（阈值 1 且已有 1 次上游失败）", got.Status)
	}
}
