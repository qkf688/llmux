package common

import "testing"

// TestIsPassingStatus 锁住 Stage 5 统一的通过口径：**仅 200 与 201**，其余一律不通过。
// 201 是这次统一的动因（兼容网关回 201 曾被判死并自动停用关联）；204/206/299 刻意
// 排除在外——它们不是聊天补全该有的回答，判通过会让空正文走成功副作用（审查回批）。
func TestIsPassingStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{"199 低于成功区间", 199, false},
		{"200 经典成功", 200, true},
		{"201 兼容网关的成功码", 201, true},
		{"202 未处理完成不属通过集", 202, false},
		{"204 成功但无正文不属通过集", 204, false},
		{"206 部分内容不属通过集", 206, false},
		{"299 成功区间末端不属通过集", 299, false},
		{"300 多选项不属通过集", 300, false},
		{"301 重定向不属通过集", 301, false},
		{"401 鉴权失败", 401, false},
		{"429 限流", 429, false},
		{"500 上游故障", 500, false},
		{"0 未拿到响应", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPassingStatus(tc.status); got != tc.want {
				t.Fatalf("IsPassingStatus(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
