package testapi

import (
	"strings"
	"testing"
)

func TestSelectScenarioIndex(t *testing.T) {
	t.Run("stable default", func(t *testing.T) {
		idx, err := selectScenarioIndex("model-123", "", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx < 0 || idx >= 3 {
			t.Fatalf("index out of range: %d", idx)
		}
		idx2, err := selectScenarioIndex("model-123", "", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx2 != idx {
			t.Fatalf("expected stable index, got %d then %d", idx, idx2)
		}
	})

	t.Run("explicit index", func(t *testing.T) {
		idx, err := selectScenarioIndex("model-123", "1", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 1 {
			t.Fatalf("idx = %d, want %d", idx, 1)
		}
	})

	t.Run("invalid index", func(t *testing.T) {
		if _, err := selectScenarioIndex("model-123", "not-a-number", 3); err == nil {
			t.Fatal("expected error")
		}
		if _, err := selectScenarioIndex("model-123", "9", 3); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNormalizeCity(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "南京", want: "南京"},
		{raw: "南京市", want: "南京"},
		{raw: "Nanjing", want: "南京"},
		{raw: "Beijing, China", want: "北京"},
		{raw: "广州", want: "广州"},
		{raw: "unknown-city", want: "unknown-city"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			if got := normalizeCity(tc.raw); got != tc.want {
				t.Fatalf("normalizeCity(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestValidateReactToolCalls(t *testing.T) {
	tests := []struct {
		name         string
		toolCities   []string
		expected     []string
		expectErrSub string
	}{
		{
			name:       "ok any order",
			toolCities: []string{"北京", "南京"},
			expected:   []string{"南京", "北京"},
		},
		{
			name:       "ok extra calls",
			toolCities: []string{"南京", "北京", "南京"},
			expected:   []string{"南京", "北京"},
		},
		{
			name:         "missing city",
			toolCities:   []string{"南京"},
			expected:     []string{"南京", "北京"},
			expectErrSub: "次数过少",
		},
		{
			name:         "missing city with enough calls",
			toolCities:   []string{"南京", "南京"},
			expected:     []string{"南京", "北京"},
			expectErrSub: "缺少城市",
		},
		{
			name:         "unexpected city",
			toolCities:   []string{"南京", "杭州"},
			expected:     []string{"南京", "北京"},
			expectErrSub: "非预期城市",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateReactToolCalls(tc.toolCities, tc.expected)
			if tc.expectErrSub == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.expectErrSub)
				}
				if !strings.Contains(err.Error(), tc.expectErrSub) {
					t.Fatalf("error = %q, want contains %q", err.Error(), tc.expectErrSub)
				}
			}
		})
	}
}

func TestValidateReactFinalResponse(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		expected     []string
		expectErrSub string
	}{
		{
			name:     "ok chinese",
			text:     "南京天气晴转多云，温度 18℃；北京天气大雨转小雨，温度 15℃。",
			expected: []string{"南京", "北京"},
		},
		{
			name:     "ok english",
			text:     "Beijing weather is rainy; Nanjing weather is cloudy. Temperature details above.",
			expected: []string{"南京", "北京"},
		},
		{
			name:         "missing city",
			text:         "南京天气不错，温度 18℃。",
			expected:     []string{"南京", "北京"},
			expectErrSub: "缺少城市",
		},
		{
			name:         "missing weather keyword",
			text:         "南京和北京我都看过了。",
			expected:     []string{"南京", "北京"},
			expectErrSub: "缺少天气",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateReactFinalResponse(tc.text, tc.expected)
			if tc.expectErrSub == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.expectErrSub)
				}
				if !strings.Contains(err.Error(), tc.expectErrSub) {
					t.Fatalf("error = %q, want contains %q", err.Error(), tc.expectErrSub)
				}
			}
		})
	}
}

func TestBuildWeatherStub(t *testing.T) {
	got, ok := buildWeatherStub("北京", "fahrenheit")
	if !ok {
		t.Fatal("expected ok")
	}
	if !strings.Contains(got, "°F") {
		t.Fatalf("expected fahrenheit output, got: %s", got)
	}
}

func TestWeatherFromArguments(t *testing.T) {
	city, res := weatherFromArguments(`{"location":"南京市","unit":"celsius"}`)
	if city != "南京" {
		t.Fatalf("city = %q, want %q", city, "南京")
	}
	if !strings.Contains(res, "南京") || !strings.Contains(res, "℃") {
		t.Fatalf("unexpected response: %s", res)
	}
}
