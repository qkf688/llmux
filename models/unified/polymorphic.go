package unified

import (
	"encoding/json"
	"errors"
)

// UnifiedStop 停止序列 (支持 string 或 []string)。
type UnifiedStop struct {
	Single   *string  `json:"-"`
	Multiple []string `json:"-"`
}

// MarshalJSON 自定义 JSON 序列化。
func (s UnifiedStop) MarshalJSON() ([]byte, error) {
	if s.Single != nil {
		return json.Marshal(s.Single)
	}
	if len(s.Multiple) > 0 {
		return json.Marshal(s.Multiple)
	}
	return []byte("null"), nil
}

// UnmarshalJSON 自定义 JSON 反序列化。
func (s *UnifiedStop) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		s.Single = &str
		return nil
	}

	var strs []string
	err = json.Unmarshal(data, &strs)
	if err == nil {
		s.Multiple = strs
		return nil
	}

	return errors.New("invalid stop type: must be string or string array")
}

// UnifiedEmbeddingInput 嵌入输入 (参考 Octopus)。
type UnifiedEmbeddingInput struct {
	Single   *string
	Multiple []string
}

// MarshalJSON 自定义 JSON 序列化。
func (i UnifiedEmbeddingInput) MarshalJSON() ([]byte, error) {
	if i.Single != nil {
		return json.Marshal(i.Single)
	}
	if len(i.Multiple) > 0 {
		return json.Marshal(i.Multiple)
	}
	return []byte("null"), nil
}

// UnmarshalJSON 自定义 JSON 反序列化。
func (i *UnifiedEmbeddingInput) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		i.Single = &str
		return nil
	}

	var strs []string
	err = json.Unmarshal(data, &strs)
	if err == nil {
		i.Multiple = strs
		return nil
	}

	return errors.New("invalid embedding input type")
}
