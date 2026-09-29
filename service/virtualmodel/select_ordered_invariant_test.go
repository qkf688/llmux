package virtualmodel

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// SelectRealModelsOrdered 的**出口不变式**：只要返回 err == nil，列表必非空。
//
// 该保证是 chat 主路径删掉 `len(orderedModels) == 0` 分支的依据
// （service/chat/chat_provider_meta.go 直接取 [0]，见 Stage 7）。它来自
// loadCandidatePool：无启用映射 / 真实模型缺失 / 全被拉黑 三条空池路径都在那里
// 报错，而各策略按 mapping 逐一构建、且「mapping 的模型必在 modelByID 里」是同一
// 函数内的成对写入不变式。
//
// 若是新增 selector、或给 select_ordered 的 findModelByID 加跳过条件（如临时熔断），
// 本用例即红——否则回归形态是主路径 orderedModels[0] panic（Recovery 兜成 500），
// 而原实现是干净的错误返回。
func TestSelectRealModelsOrdered_NonEmptyWhenNoError(t *testing.T) {
	db := testutil.New(t,
		&models.Model{},
		&models.Provider{},
		&models.ModelWithProvider{},
		&models.VirtualModel{},
		&models.VirtualModelMapping{},
	)

	s := &Service{db: db, roundRobinState: make(map[uint]int)}
	ctx := context.Background()

	enabled := true
	provider := models.Provider{Name: "p", Type: "openai", Config: "{}"}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	realModels := make([]models.Model, 0, 3)
	for _, name := range []string{"rm-1", "rm-2", "rm-3"} {
		m := models.Model{Name: name}
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("create real model %s: %v", name, err)
		}
		realModels = append(realModels, m)
	}

	// 中间那个真实模型刻意不给关联：候选池过滤只剔除「被拉黑供应商」的模型，
	// 没有关联 ≠ 出池，故它仍应出现在有序列表里（provider 级选路时才淘汰）。
	for _, m := range []models.Model{realModels[0], realModels[2]} {
		if err := db.Create(&models.ModelWithProvider{
			ModelID: m.ID, ProviderID: provider.ID, ProviderModel: m.Name, Status: &enabled,
		}).Error; err != nil {
			t.Fatalf("create association for %s: %v", m.Name, err)
		}
	}

	for _, strategy := range []string{"priority", "round_robin", "random", "unknown-strategy"} {
		t.Run(strategy, func(t *testing.T) {
			virtual := models.VirtualModel{Name: "vm-" + strategy, Strategy: strategy, Enabled: &enabled}
			if err := db.Create(&virtual).Error; err != nil {
				t.Fatalf("create virtual model: %v", err)
			}
			for _, m := range realModels {
				if err := db.Create(&models.VirtualModelMapping{
					VirtualModelID: virtual.ID, RealModelID: m.ID, Enabled: &enabled, Weight: 1, Priority: 1,
				}).Error; err != nil {
					t.Fatalf("create mapping: %v", err)
				}
			}

			ordered, err := s.SelectRealModelsOrdered(ctx, &virtual)
			if err != nil {
				t.Fatalf("SelectRealModelsOrdered error: %v", err)
			}
			if len(ordered) == 0 {
				t.Fatalf("有序列表为空且 err == nil：主路径取 [0] 将 panic，不变式被破坏")
			}
			if len(ordered) != len(realModels) {
				t.Fatalf("len(ordered) = %d, want %d（策略不得中途丢项）", len(ordered), len(realModels))
			}
		})
	}
}
