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
