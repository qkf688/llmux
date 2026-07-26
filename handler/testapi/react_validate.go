package testapi

import (
	"errors"
	"fmt"
	"strings"
)

func normalizeCity(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, "市")
	s = strings.ToLower(s)

	switch {
	case strings.Contains(s, "南京") || strings.Contains(s, "nanjing"):
		return "南京"
	case strings.Contains(s, "北京") || strings.Contains(s, "beijing"):
		return "北京"
	case strings.Contains(s, "上海") || strings.Contains(s, "shanghai"):
		return "上海"
	case strings.Contains(s, "广州") || strings.Contains(s, "guangzhou"):
		return "广州"
	case strings.Contains(s, "深圳") || strings.Contains(s, "shenzhen"):
		return "深圳"
	case strings.Contains(s, "杭州") || strings.Contains(s, "hangzhou"):
		return "杭州"
	default:
		return strings.TrimSpace(raw)
	}
}

func validateReactToolCalls(toolCities []string, expectedCities []string) error {
	if len(expectedCities) == 0 {
		return errors.New("no expected cities")
	}
	if len(toolCities) < len(expectedCities) {
		return fmt.Errorf("工具调用次数过少: 期望至少 %d 次, 实际 %d 次", len(expectedCities), len(toolCities))
	}

	expected := make(map[string]int, len(expectedCities))
	for _, city := range expectedCities {
		expected[city] = 0
	}

	unknown := make([]string, 0)
	for _, city := range toolCities {
		if _, ok := expected[city]; ok {
			expected[city]++
			continue
		}
		if strings.TrimSpace(city) != "" {
			unknown = append(unknown, city)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("工具调用包含非预期城市: %s", strings.Join(unknown, ", "))
	}

	missing := make([]string, 0)
	for _, city := range expectedCities {
		if expected[city] == 0 {
			missing = append(missing, city)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("工具调用缺少城市: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateReactFinalResponse(finalResponse string, expectedCities []string) error {
	text := strings.TrimSpace(finalResponse)
	if text == "" {
		return errors.New("模型未返回有效文本内容")
	}
	lower := strings.ToLower(text)

	missing := make([]string, 0)
	for _, city := range expectedCities {
		if !containsCityAlias(lower, city) {
			missing = append(missing, city)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("最终回复缺少城市信息: %s", strings.Join(missing, ", "))
	}

	if !(strings.Contains(text, "天气") || strings.Contains(text, "温度") || strings.Contains(lower, "weather") || strings.Contains(text, "℃") || strings.Contains(text, "°")) {
		return errors.New("最终回复缺少天气相关信息")
	}
	return nil
}

func containsCityAlias(lowerText, canonicalCity string) bool {
	if strings.Contains(lowerText, strings.ToLower(canonicalCity)) {
		return true
	}
	switch canonicalCity {
	case "南京":
		return strings.Contains(lowerText, "nanjing")
	case "北京":
		return strings.Contains(lowerText, "beijing")
	case "上海":
		return strings.Contains(lowerText, "shanghai")
	case "广州":
		return strings.Contains(lowerText, "guangzhou")
	case "深圳":
		return strings.Contains(lowerText, "shenzhen")
	case "杭州":
		return strings.Contains(lowerText, "hangzhou")
	default:
		return false
	}
}
