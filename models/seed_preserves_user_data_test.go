package models

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/internal/testutil"
	"gorm.io/gorm"
)

// 本文件钉住两条「启动种子不得覆盖用户当前数据」的回归。
//
// 背景（AGENTS.md §0「不背历史包袱」的推论边界）：§0.1 禁的是「为兼容旧数据新增
// 机制」，不是「可以无条件覆盖用户现在的数据」。历史上 seed() 里有两条按「存量行」
// 写的无条件 UPDATE，每次启动都跑：一条把 priority=0/NULL 的关联重置为默认值，
// 一条把 request_total_timeout 刷回 schema 默认值。两者都把「历史遗留的空值」
// 与「当前代码或用户正在用的合法值」混为一谈——
//   · priority=0 是 service/adjustment 的权重衰减**设计上**要产生的值（DecayPriority
//     的 floor 就是 0），启动时被拉回默认值等于每次重启都抹掉衰减系统的成果；
//   · request_total_timeout 是设置页里用户可改的项（schema 带 Min 约束），
//     无条件覆盖等于每次重启都静默丢弃用户的调整。

// testSeedDB 建一张够跑 seed() 的库并把全局 DB 指过去。
func testSeedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.New(t, &Setting{}, &Model{}, &ModelWithProvider{})
	prev := DB
	DB = db
	t.Cleanup(func() { DB = prev })
	return db
}

// TestSeed_PreservesDecayedPriority 锁「重启不抹掉权重衰减成果」。
// priority=0 由运行时衰减产生，是合法状态，不该被启动种子当「未设置」拉回默认值。
func TestSeed_PreservesDecayedPriority(t *testing.T) {
	db := testSeedDB(t)
	ctx := context.Background()

	assoc := &ModelWithProvider{ModelID: 1, ProviderID: 1, ProviderModel: "gpt-x", Priority: 0}
	if err := db.Create(assoc).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	seed(ctx)

	var got ModelWithProvider
	if err := db.First(&got, assoc.ID).Error; err != nil {
		t.Fatalf("reload association: %v", err)
	}
	if got.Priority != 0 {
		t.Fatalf("priority = %d, want 0：权重衰减打到 0 是合法状态，启动种子不得拉回默认值", got.Priority)
	}
}

// TestSeed_PreservesUserEditedRequestTotalTimeout 锁「重启不覆盖用户改过的超时」。
// 该设置在设置页可改，seed 不得每次启动把它刷回 schema 默认值。
func TestSeed_PreservesUserEditedRequestTotalTimeout(t *testing.T) {
	db := testSeedDB(t)
	ctx := context.Background()

	// 首次 seed 建默认行
	seed(ctx)
	const userValue = "123"
	if err := db.Model(&Setting{}).Where("key = ?", SettingKeyRequestTotalTimeout).
		Update("value", userValue).Error; err != nil {
		t.Fatalf("模拟用户改值: %v", err)
	}

	// 再次 seed（等价于重启）
	seed(ctx)

	var got Setting
	if err := db.Where("key = ?", SettingKeyRequestTotalTimeout).First(&got).Error; err != nil {
		t.Fatalf("reload setting: %v", err)
	}
	if got.Value != userValue {
		t.Fatalf("request_total_timeout = %q, want %q：用户改过的值不得被启动种子覆盖", got.Value, userValue)
	}
}
