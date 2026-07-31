package models

import (
	"context"
	"path/filepath"
	"testing"
)

// TestSettingSchemaFieldsUnique 守护 SettingSchemaForField 的隐含前提：
// Field 名在全表唯一。重名会让按字段名查找命中任意一条（map 遍历无序），
// UpdateSettingsRequest 的反射写入就会把值写进别的 key。
func TestSettingSchemaFieldsUnique(t *testing.T) {
	keysByField := make(map[string][]string)
	for key, schema := range SettingSchemas() {
		if schema.Field == "" {
			t.Errorf("schema %s has empty Field", key)
			continue
		}
		keysByField[schema.Field] = append(keysByField[schema.Field], key)
	}

	for field, keys := range keysByField {
		if len(keys) > 1 {
			t.Errorf("Field %q is shared by %d keys: %v", field, len(keys), keys)
		}
	}
}

// TestSeedCreatesAllSchemaKeys 回归：seed 只遍历 SettingSchemas，
// 缺席的键不会建行；而健康检测设置的更新是 UPDATE-only，
// 行不存在时静默 no-op（返回 200 但存不进去）。
func TestSeedCreatesAllSchemaKeys(t *testing.T) {
	Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test.db"))
	t.Cleanup(func() {
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	var rows []Setting
	if err := DB.Find(&rows).Error; err != nil {
		t.Fatalf("list settings: %v", err)
	}
	seeded := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		seeded[row.Key] = struct{}{}
	}

	for key := range SettingSchemas() {
		if _, ok := seeded[key]; !ok {
			t.Errorf("schema key %q was not seeded", key)
		}
	}
}
