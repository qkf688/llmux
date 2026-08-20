package shared

import (
	"bytes"
	"encoding/json"
)

// 本文件提供入站请求 DTO 的字段容器：**宽容解析 + 三态**。
//
// 为什么所有 UnmarshalJSON 都吞掉错误而不上报：网关的入站解析在 map 时代对类型不
// 匹配是静默 skip（`req["temperature"].(float64)` 断言失败就当没传）。若换成裸
// `float64` 字段，客户端发 `"temperature": "0.5"` 会让**整条请求 400**——这是对外
// 行为变更，而网关的价值之一就是比上游更宽容。所以容器只负责「能解就取，不能解就
// 当没传」，把「类型不对」与「没传」统一成 Set=false。
//
// Set 是必需的第三态：Value 的零值（0 / "" / false）与「客户端显式传了零值」不可
// 区分。少了 Set，`temperature: 0` 会被当成没传，出站时静默丢失。
//
// 本文件刻意放在 shared 而非各协议子包：openai 与 anthropic 入站都需要同一套语义，
// 复制两份必然漂移（AGENTS.md 4.4 DRY）。shared 只依赖标准库，不反向依赖业务包。

// IsJSONNull 判断 data 是否为 JSON 字面量 null。
//
// null 与「键不存在」在本项目里同义（都表示未设置），故容器遇到 null 直接返回、
// 保持 Set=false，而不是把零值当成用户显式设置。
func IsJSONNull(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

// Number 约束 OptionalNumber 可承载的数值类型。
type Number interface {
	~int | ~int64 | ~float64
}

// Optional 承载非数值标量（string / bool 等）的可选入站字段。
//
// 数值**不要**用 Optional：见 OptionalNumber 的注释。
type Optional[T any] struct {
	Value T
	Set   bool
}

func (f *Optional[T]) UnmarshalJSON(data []byte) error {
	if IsJSONNull(data) {
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

// OptionalNumber 承载数值型可选入站字段，**统一先解成 float64 再转目标类型**。
//
// 不能直接 `json.Unmarshal` 到 int / int64：标准库拒绝把 `8192.0` 解进整型字段，
// 而 map 时代所有 JSON 数字都先变成 float64 再 int() 转换，`{"max_tokens": 8192.0}`
// 是能正常工作的。走 float64 中转是为了保住这个既有宽容度。
type OptionalNumber[T Number] struct {
	Value T
	Set   bool
}

func (f *OptionalNumber[T]) UnmarshalJSON(data []byte) error {
	if IsJSONNull(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = T(value)
		f.Set = true
	}
	return nil
}

// OptionalStringSeq 承载字符串数组，**逐元素过滤**非字符串项。
//
// 与 Optional[[]string] 的区别：后者遇到 `["a", 1]` 会整体解析失败、字段全丢；
// 本类型保留 "a" 丢掉 1，与 maputil.StringSlice 的既有行为一致。
type OptionalStringSeq struct {
	Value []string
	Set   bool
}

func (f *OptionalStringSeq) UnmarshalJSON(data []byte) error {
	if IsJSONNull(data) {
		return nil
	}
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, value := range raw {
		if str, ok := value.(string); ok {
			result = append(result, str)
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

// OptionalStringMap 承载字符串字典，逐值过滤非字符串项（理由同 OptionalStringSeq）。
type OptionalStringMap struct {
	Value map[string]string
	Set   bool
}

func (f *OptionalStringMap) UnmarshalJSON(data []byte) error {
	if IsJSONNull(data) {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]string, len(raw))
	for key, value := range raw {
		if str, ok := value.(string); ok {
			result[key] = str
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

// OptionalInt64Map 承载整数字典，逐值过滤非数值项（理由同 OptionalStringSeq）。
type OptionalInt64Map struct {
	Value map[string]int64
	Set   bool
}

func (f *OptionalInt64Map) UnmarshalJSON(data []byte) error {
	if IsJSONNull(data) {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]int64, len(raw))
	for key, value := range raw {
		if num, ok := value.(float64); ok {
			result[key] = int64(num)
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

// RawArray 承载「元素形态多态、需要二次分派」的数组（messages / tools / content 块）。
//
// 不做元素级解析是刻意的：这些数组的元素是 6 种以上 type 混排，形态判别属于各协议
// 子包的职责，容器只保证「不是数组就当没传」这一层宽容。
type RawArray []json.RawMessage

func (a *RawArray) UnmarshalJSON(data []byte) error {
	if IsJSONNull(data) {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		*a = raw
	}
	return nil
}

// RawJSONValue 把一段原始 JSON 解成 interface{}（map / slice / 标量），失败返回 nil。
//
// 供「DTO 停在 json.RawMessage、内部仍走 map 逐键解析」的过渡形态使用：顶层字段已
// struct 化以便反射认领键，嵌套多态结构则原样交给既有 map 解析逻辑，避免为了 struct
// 化而重写多态分派。
func RawJSONValue(data json.RawMessage) interface{} {
	if len(data) == 0 {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	return value
}
