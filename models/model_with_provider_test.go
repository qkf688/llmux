package models

import "testing"

// TestModelWithProvider_SupportsThinkingResolved 覆盖 SupportsThinkingResolved 全部分支：
// override 优先于 model 继承；model 为 nil 或 model 不支持时兜底 false，不得 panic。
func TestModelWithProvider_SupportsThinkingResolved(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	modelTrue := &Model{SupportsThinking: true}
	modelFalse := &Model{SupportsThinking: false}

	tests := []struct {
		name     string
		mwp      *ModelWithProvider
		model    *Model
		expected bool
	}{
		{"association nil + model nil → false", &ModelWithProvider{SupportsThinking: nil}, nil, false},
		{"association nil + model true → true", &ModelWithProvider{SupportsThinking: nil}, modelTrue, true},
		{"association nil + model false → false", &ModelWithProvider{SupportsThinking: nil}, modelFalse, false},
		{"association true override + model false → true", &ModelWithProvider{SupportsThinking: boolPtr(true)}, modelFalse, true},
		{"association false override + model true → false", &ModelWithProvider{SupportsThinking: boolPtr(false)}, modelTrue, false},
		{"association true override + model nil → true", &ModelWithProvider{SupportsThinking: boolPtr(true)}, nil, true},
		{"receiver nil + model true → true（方法内部兜底）", nil, modelTrue, true},
		{"receiver nil + model nil → false（不 panic）", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mwp.SupportsThinkingResolved(tt.model)
			if got != tt.expected {
				t.Errorf("SupportsThinkingResolved() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestModelWithProvider_ThinkingLevelsResolved 覆盖 ThinkingLevelsResolved 全部分支：
// override（*[]string）非 nil 时以 override 为准（含空切片=显式不约束）；
// nil 时继承 model 的 ThinkingLevels；receiver/model 为 nil 时兜底 nil，不 panic。
func TestModelWithProvider_ThinkingLevelsResolved(t *testing.T) {
	strSlicePtr := func(v []string) *[]string { return &v }

	modelWithLevels := &Model{ThinkingLevels: []string{"low", "high"}}
	modelNoLevels := &Model{ThinkingLevels: nil}

	tests := []struct {
		name     string
		mwp      *ModelWithProvider
		model    *Model
		expected []string
	}{
		{"override 非空 → override", &ModelWithProvider{ThinkingLevels: strSlicePtr([]string{"medium", "max"})}, modelWithLevels, []string{"medium", "max"}},
		{"override 空切片 → 显式不约束（空切片）", &ModelWithProvider{ThinkingLevels: strSlicePtr([]string{})}, modelWithLevels, []string{}},
		{"override nil + model 有白名单 → 继承", &ModelWithProvider{ThinkingLevels: nil}, modelWithLevels, []string{"low", "high"}},
		{"override nil + model 无白名单 → nil", &ModelWithProvider{ThinkingLevels: nil}, modelNoLevels, nil},
		{"override nil + model nil → nil", &ModelWithProvider{ThinkingLevels: nil}, nil, nil},
		{"receiver nil + model 有白名单 → 继承（方法内部兜底）", nil, modelWithLevels, []string{"low", "high"}},
		{"receiver nil + model nil → nil（不 panic）", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mwp.ThinkingLevelsResolved(tt.model)
			if !sliceEqual(got, tt.expected) {
				t.Errorf("ThinkingLevelsResolved() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
