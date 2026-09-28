package models

import (
	"context"
	"path/filepath"
	"testing"
)

func initSettingReaderTestDB(t *testing.T) {
	t.Helper()
	Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test.db"))
	t.Cleanup(func() {
		if DB == nil {
			return
		}
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestGetSettingIntE_UnsetReturnsDefaultNoError 未设置是有效状态：key 不存在 → 默认值 + 无错。
func TestGetSettingIntE_UnsetReturnsDefaultNoError(t *testing.T) {
	initSettingReaderTestDB(t)

	v, err := GetSettingIntE(context.Background(), "no_such_key", 7, 1)
	if err != nil {
		t.Fatalf("err = %v, want nil（key 不存在不是读失败）", err)
	}
	if v != 7 {
		t.Fatalf("v = %d, want 7", v)
	}
}

// TestGetSettingIntE_QueryFailureReturnsError 查询失败（ctx 已死 = 结算预算中途到期的
// 同类形态）→ err 非 nil：default 不可信，调用方必须显式降级（Stage 33 核心语义）。
func TestGetSettingIntE_QueryFailureReturnsError(t *testing.T) {
	initSettingReaderTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := GetSettingIntE(ctx, "some_key", 3, 1); err == nil {
		t.Fatal("err = nil, want 非 nil（查询失败不得伪装成取默认值）")
	}
}

// TestGetSettingIntE_InvalidValueReturnsError 存值非法（解析失败 / 低于下限）同按读失败处理。
func TestGetSettingIntE_InvalidValueReturnsError(t *testing.T) {
	initSettingReaderTestDB(t)

	if err := DB.Create(&Setting{Key: "bad_int", Value: "abc"}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}
	if _, err := GetSettingIntE(context.Background(), "bad_int", 3, 1); err == nil {
		t.Fatal("err = nil, want 非 nil（解析失败不得退默认值静默续行）")
	}

	if err := DB.Create(&Setting{Key: "below_min", Value: "0"}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}
	if _, err := GetSettingIntE(context.Background(), "below_min", 3, 1); err == nil {
		t.Fatal("err = nil, want 非 nil（低于 minValue 同按读失败处理）")
	}
}

// TestGetSettingIntE_ValidValueNoError 合法存值 → 原值 + 无错。
func TestGetSettingIntE_ValidValueNoError(t *testing.T) {
	initSettingReaderTestDB(t)

	if err := DB.Create(&Setting{Key: "good_int", Value: "42"}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}
	v, err := GetSettingIntE(context.Background(), "good_int", 3, 1)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if v != 42 {
		t.Fatalf("v = %d, want 42", v)
	}
}

// TestGetSettingBoolE_UnsetQueryFailure 两个失败态分开：未设置 → 默认值无错；查询失败 → err。
func TestGetSettingBoolE_UnsetQueryFailure(t *testing.T) {
	initSettingReaderTestDB(t)

	v, err := GetSettingBoolE(context.Background(), "no_such_bool", true)
	if err != nil || !v {
		t.Fatalf("unset: (v, err) = (%v, %v), want (true, nil)", v, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetSettingBoolE(ctx, "some_bool", false); err == nil {
		t.Fatal("err = nil, want 非 nil（查询失败不得伪装成取默认值）")
	}
}

// TestGetSettingPlain_IgnoresErrorForLegacyPaths 裸变体保持旧语义（读失败照退默认值），
// 供非写决策点沿用；E 变体的行为差异不改变裸变体契约。
func TestGetSettingPlain_IgnoresErrorForLegacyPaths(t *testing.T) {
	initSettingReaderTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if v := GetSettingInt(ctx, "some_key", 9, 1); v != 9 {
		t.Fatalf("v = %d, want 9（裸变体维持退默认值语义）", v)
	}
	if v := GetSettingBool(ctx, "some_key", true); !v {
		t.Fatal("v = false, want true（裸变体维持退默认值语义）")
	}
}
