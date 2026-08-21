package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSetAnthropicBeta_EmptyDeletesKey 覆盖 anthropic-beta 头的两态语义。
//
// 为什么必须有这条：改前两处调用点都是无条件 Set("anthropic-beta", a.Beta)，而配置模板的
// beta 默认是空串（providers/meta_bodies.go），于是绝大多数请求都携带一个空值 anthropic-beta 头；
// 且关联开启 withHeader 时客户端自带的 anthropic-beta 会被 Clone 进来（chatcore.BuildHeaders），
// 空值 Set 恰好把它盖成空串而非清除——两种情况都不等于「本供应商不启用任何 beta」。
func TestSetAnthropicBeta_EmptyDeletesKey(t *testing.T) {
	tests := []struct {
		name        string
		beta        string
		presetValue string // 模拟 withHeader=true 时客户端头被 Clone 进来的残留值
		wantExists  bool
		wantValue   string
	}{
		{
			name:       "beta 空 + 无残留 → 不写该头",
			beta:       "",
			wantExists: false,
		},
		{
			name:        "beta 空 + 客户端残留 → 清掉残留（运维配置权威）",
			beta:        "",
			presetValue: "interleaved-thinking-2025-05-14",
			wantExists:  false,
		},
		{
			name:       "beta 非空 → 写入配置值",
			beta:       "output-128k-2025-02-19",
			wantExists: true,
			wantValue:  "output-128k-2025-02-19",
		},
		{
			name:        "beta 非空 + 客户端残留 → 配置值覆盖残留",
			beta:        "output-128k-2025-02-19",
			presetValue: "interleaved-thinking-2025-05-14",
			wantExists:  true,
			wantValue:   "output-128k-2025-02-19",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.presetValue != "" {
				header.Set("anthropic-beta", tt.presetValue)
			}

			setAnthropicBeta(header, tt.beta)

			values, exists := header["Anthropic-Beta"]
			if exists != tt.wantExists {
				t.Fatalf("anthropic-beta 键存在性 = %v (values=%v), want %v", exists, values, tt.wantExists)
			}
			if tt.wantExists && header.Get("anthropic-beta") != tt.wantValue {
				t.Errorf("anthropic-beta = %q, want %q", header.Get("anthropic-beta"), tt.wantValue)
			}
		})
	}
}

// TestAnthropicBuildReq_BetaHeader 打在出站 *http.Request 上：setAnthropicBeta 单测通过
// 不等于 BuildReq 真的用了它（此前是内联 Set）。顺带守住 anthropic-version 恒写入。
func TestAnthropicBuildReq_BetaHeader(t *testing.T) {
	tests := []struct {
		name       string
		beta       string
		inHeader   http.Header
		wantExists bool
		wantValue  string
	}{
		{
			name:       "beta 空 + header nil",
			beta:       "",
			inHeader:   nil,
			wantExists: false,
		},
		{
			name:       "beta 空 + 客户端头带 beta（withHeader=true 场景）",
			beta:       "",
			inHeader:   http.Header{"Anthropic-Beta": []string{"interleaved-thinking-2025-05-14"}},
			wantExists: false,
		},
		{
			name:       "beta 已配置",
			beta:       "output-128k-2025-02-19",
			inHeader:   nil,
			wantExists: true,
			wantValue:  "output-128k-2025-02-19",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Anthropic{BaseURL: "https://example.invalid", APIKey: "k", Version: "2023-06-01", Beta: tt.beta}

			req, err := a.BuildReq(context.Background(), tt.inHeader, "claude-sonnet-4-6", []byte(`{"max_tokens":1024}`))
			if err != nil {
				t.Fatalf("BuildReq 失败: %v", err)
			}

			values, exists := req.Header["Anthropic-Beta"]
			if exists != tt.wantExists {
				t.Fatalf("anthropic-beta 键存在性 = %v (values=%v), want %v", exists, values, tt.wantExists)
			}
			if tt.wantExists && req.Header.Get("anthropic-beta") != tt.wantValue {
				t.Errorf("anthropic-beta = %q, want %q", req.Header.Get("anthropic-beta"), tt.wantValue)
			}
			if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
				t.Errorf("anthropic-version = %q, want 2023-06-01", got)
			}
		})
	}
}

// TestAnthropicHasBetaFeature 覆盖 beta 特性查询：上层的 budget/max_tokens 收敛靠它
// 识别 interleaved thinking 例外，判错的后果是静默改写合法请求（不报错、只是思考变浅）。
func TestAnthropicHasBetaFeature(t *testing.T) {
	const interleaved = "interleaved-thinking-2025-05-14"

	for _, tt := range []struct {
		name string
		beta string
		want bool
	}{
		{name: "未配 beta", beta: "", want: false},
		{name: "单值精确命中", beta: interleaved, want: true},
		{name: "多值列表命中", beta: "output-128k-2025-02-19," + interleaved, want: true},
		{name: "多值带空格命中（运维手填常见形态）", beta: "output-128k-2025-02-19, " + interleaved, want: true},
		{name: "大小写不敏感", beta: "Interleaved-Thinking-2025-05-14", want: true},
		{name: "配了别的 beta 不误命中", beta: "output-128k-2025-02-19", want: false},
		{name: "前缀相同但非同一特性不误命中", beta: "interleaved-thinking-2024-01-01", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &Anthropic{Beta: tt.beta}
			if got := a.HasBetaFeature(interleaved); got != tt.want {
				t.Errorf("HasBetaFeature(%q) with Beta=%q = %v, want %v", interleaved, tt.beta, got, tt.want)
			}
		})
	}

	// 空特性名必须返回 false：否则「未指定要查什么」会被任意 Beta 配置误判成命中。
	t.Run("空特性名", func(t *testing.T) {
		a := &Anthropic{Beta: interleaved}
		if a.HasBetaFeature("") {
			t.Error(`HasBetaFeature("") = true, want false`)
		}
	})

	// 断言 *Anthropic 满足能力接口：上层做的是接口断言，实现签名漂移时这里先红。
	t.Run("实现 BetaFeatureCapable", func(t *testing.T) {
		var _ BetaFeatureCapable = &Anthropic{}
	})
}

// TestAnthropicModels_BetaHeader 覆盖拉模型列表这条独立路径：它自建 header，
// 与 BuildReq 不共用代码，改一处漏一处正是此前的实际形态。
func TestAnthropicModels_BetaHeader(t *testing.T) {
	for _, tt := range []struct {
		name       string
		beta       string
		wantExists bool
	}{
		{name: "beta 空 → 不写该头", beta: "", wantExists: false},
		{name: "beta 非空 → 写入", beta: "output-128k-2025-02-19", wantExists: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var gotHeader http.Header
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[]}`))
			}))
			defer srv.Close()

			a := &Anthropic{BaseURL: srv.URL, APIKey: "k", Version: "2023-06-01", Beta: tt.beta}
			if _, err := a.Models(context.Background()); err != nil {
				t.Fatalf("Models 失败: %v", err)
			}

			values, exists := gotHeader["Anthropic-Beta"]
			if exists != tt.wantExists {
				t.Fatalf("anthropic-beta 键存在性 = %v (values=%v), want %v", exists, values, tt.wantExists)
			}
			if tt.wantExists && gotHeader.Get("anthropic-beta") != tt.beta {
				t.Errorf("anthropic-beta = %q, want %q", gotHeader.Get("anthropic-beta"), tt.beta)
			}
		})
	}
}
