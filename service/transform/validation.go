package transform

import (
	"encoding/json"
	"errors"
	"fmt"
)

// 参数验证函数
// 参考: E:\a-2025_12-projects\octopus\internal\transformer\

// validateTemperature 验证 temperature 参数范围 (0-2)
func validateTemperature(temp *float64) error {
	if temp == nil {
		return nil
	}
	if *temp < 0 || *temp > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got %f", *temp)
	}
	return nil
}

// validateTopP 验证 top_p 参数范围 (0-1)
func validateTopP(topP *float64) error {
	if topP == nil {
		return nil
	}
	if *topP < 0 || *topP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1, got %f", *topP)
	}
	return nil
}

// validateFrequencyPenalty 验证 frequency_penalty 参数范围 (-2 to 2)
func validateFrequencyPenalty(penalty *float64) error {
	if penalty == nil {
		return nil
	}
	if *penalty < -2 || *penalty > 2 {
		return fmt.Errorf("frequency_penalty must be between -2 and 2, got %f", *penalty)
	}
	return nil
}

// validatePresencePenalty 验证 presence_penalty 参数范围 (-2 to 2)
func validatePresencePenalty(penalty *float64) error {
	if penalty == nil {
		return nil
	}
	if *penalty < -2 || *penalty > 2 {
		return fmt.Errorf("presence_penalty must be between -2 and 2, got %f", *penalty)
	}
	return nil
}

// validateTopLogprobs 验证 top_logprobs 参数范围 (0-20)
func validateTopLogprobs(topLogprobs *int64) error {
	if topLogprobs == nil {
		return nil
	}
	if *topLogprobs < 0 || *topLogprobs > 20 {
		return fmt.Errorf("top_logprobs must be between 0 and 20, got %d", *topLogprobs)
	}
	return nil
}

// ValidateUnifiedRequest 验证 UnifiedRequest 的所有参数
func ValidateUnifiedRequest(req *UnifiedRequest) error {
	if req == nil {
		return errors.New("request cannot be nil")
	}

	// 验证必需字段
	if req.Model == "" {
		return errors.New("model is required")
	}

	if len(req.Messages) == 0 {
		return errors.New("messages cannot be empty")
	}

	// 验证参数范围
	if err := validateTemperature(req.Temperature); err != nil {
		return err
	}

	if err := validateTopP(req.TopP); err != nil {
		return err
	}

	if err := validateFrequencyPenalty(req.FrequencyPenalty); err != nil {
		return err
	}

	if err := validatePresencePenalty(req.PresencePenalty); err != nil {
		return err
	}

	if err := validateTopLogprobs(req.TopLogprobs); err != nil {
		return err
	}

	return nil
}

// 参数修复函数

// clampFloat64 将浮点数限制在指定范围内
func clampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// clampInt64 将整数限制在指定范围内
func clampInt64(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// repairInvalidJSON 修复无效的 JSON 字符串
// 如果 JSON 无效，返回空对象 "{}"
func repairInvalidJSON(jsonStr string) string {
	if json.Valid([]byte(jsonStr)) {
		return jsonStr
	}
	return "{}"
}

// RepairUnifiedRequest 自动修复 UnifiedRequest 的参数
// 将超出范围的参数限制在有效范围内
func RepairUnifiedRequest(req *UnifiedRequest) {
	if req == nil {
		return
	}

	// 修复 temperature (0-2)
	if req.Temperature != nil {
		clamped := clampFloat64(*req.Temperature, 0, 2)
		req.Temperature = &clamped
	}

	// 修复 top_p (0-1)
	if req.TopP != nil {
		clamped := clampFloat64(*req.TopP, 0, 1)
		req.TopP = &clamped
	}

	// 修复 frequency_penalty (-2 to 2)
	if req.FrequencyPenalty != nil {
		clamped := clampFloat64(*req.FrequencyPenalty, -2, 2)
		req.FrequencyPenalty = &clamped
	}

	// 修复 presence_penalty (-2 to 2)
	if req.PresencePenalty != nil {
		clamped := clampFloat64(*req.PresencePenalty, -2, 2)
		req.PresencePenalty = &clamped
	}

	// 修复 top_logprobs (0-20)
	if req.TopLogprobs != nil {
		clamped := clampInt64(*req.TopLogprobs, 0, 20)
		req.TopLogprobs = &clamped
	}

	// 修复 max_tokens (最小为 1)
	if req.MaxTokens < 0 {
		req.MaxTokens = 1
	}

	// 修复 max_completion_tokens (最小为 1)
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens < 0 {
		minTokens := int64(1)
		req.MaxCompletionTokens = &minTokens
	}
}
