package healthcheck

import (
	"encoding/json"
	"strconv"
)

const (
	testOpenAIBody = `{
        "model": "gpt-4.1",
        "messages": [
            {
                "role": "user",
                "content": "Write a one-sentence bedtime story about a unicorn."
            }
        ]
    }`

	testOpenAIResBody = `{
        "model": "gpt-5-nano",
        "input": "Write a one-sentence bedtime story about a unicorn."
    }`

	testAnthropicBody = `{
    	"model": "claude-sonnet-4-5",
    	"max_tokens": 1000,
    	"messages": [
      		{
        		"role": "user", 
        		"content": "Write a one-sentence bedtime story about a unicorn."
      		}
    	]
 	}`
)

// HealthCheckError 健康检测错误。
type HealthCheckError struct {
	StatusCode int
	Body       string
}

func (e *HealthCheckError) Error() string {
	return "health check failed with status " + strconv.Itoa(e.StatusCode) + ": " + e.Body
}

// HealthCheckSettingsJSON 健康检测设置 JSON 结构。
type HealthCheckSettingsJSON struct {
	Enabled                 bool `json:"enabled"`
	Interval                int  `json:"interval"`
	FailureThreshold        int  `json:"failure_threshold"`
	FailureDisableEnabled   bool `json:"failure_disable_enabled"`
	AutoEnable              bool `json:"auto_enable"`
	LogRetentionCount       int  `json:"log_retention_count"`
	CountHealthCheckSuccess bool `json:"count_health_check_as_success"`
	CountHealthCheckFailure bool `json:"count_health_check_as_failure"`
	CheckDisabledOnly       bool `json:"check_disabled_only"`
}

// MarshalJSON 序列化健康检测设置。
func (s HealthCheckSettingsJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Enabled                 bool `json:"enabled"`
		Interval                int  `json:"interval"`
		FailureThreshold        int  `json:"failure_threshold"`
		FailureDisableEnabled   bool `json:"failure_disable_enabled"`
		AutoEnable              bool `json:"auto_enable"`
		LogRetentionCount       int  `json:"log_retention_count"`
		CountHealthCheckSuccess bool `json:"count_health_check_as_success"`
		CountHealthCheckFailure bool `json:"count_health_check_as_failure"`
		CheckDisabledOnly       bool `json:"check_disabled_only"`
	}{
		Enabled:                 s.Enabled,
		Interval:                s.Interval,
		FailureThreshold:        s.FailureThreshold,
		FailureDisableEnabled:   s.FailureDisableEnabled,
		AutoEnable:              s.AutoEnable,
		LogRetentionCount:       s.LogRetentionCount,
		CountHealthCheckSuccess: s.CountHealthCheckSuccess,
		CountHealthCheckFailure: s.CountHealthCheckFailure,
		CheckDisabledOnly:       s.CheckDisabledOnly,
	})
}
