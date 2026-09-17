package common

import "testing"

// TestIsPassingStatus 锁住 Stage 5 统一的通过口径：2xx 全算通过，其余全不通过。
// 201 是这次统一的动因（兼容网关回 201 曾被判死并自动停用关联），必须钉住。
func TestIsPassingStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{"199 低于成功区间", 199, false},
		{"200 经典成功", 200, true},
		{"201 兼容网关的成功码", 201, true},
		{"204 成功但无内容仍按状态码判通过", 204, true},
		{"299 区间内上界", 299, true},
		{"300 多选项不属成功", 300, false},
		{"301 重定向不属成功", 301, false},
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
