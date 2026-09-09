package common

import "testing"

// TestErrorTypeFromStatus 锁诊断测试 error_type 值域单一来源的状态码映射。
// 该函数被 handler/testapi 与 handler/pools(#24-B) 共用、并镜像到前端 webui/lib/errors.ts，
// 故在 common 直接表驱动钉死，不靠调用方委托测试间接覆盖（漂移会让前后端值域对不上）。
func TestErrorTypeFromStatus(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{"401 认证失败", 401, ErrorTypeAuth},
		{"403 认证失败", 403, ErrorTypeAuth},
		{"404 归 provider（模型/路径不存在，非鉴权）", 404, ErrorTypeProvider},
		{"400 请求校验", 400, ErrorTypeValidation},
		{"422 请求校验", 422, ErrorTypeValidation},
		{"429 限流按 4xx 校验类", 429, ErrorTypeValidation},
		{"500 上游故障", 500, ErrorTypeProvider},
		{"503 上游故障", 503, ErrorTypeProvider},
		{"3xx 非预期区间归 unknown", 302, ErrorTypeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ErrorTypeFromStatus(tc.status); got != tc.want {
				t.Fatalf("ErrorTypeFromStatus(%d) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}
