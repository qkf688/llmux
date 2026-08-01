package common

import (
	"encoding/json"
	"sort"
	"strconv"
)

// DiffEntry 表示一个字段级别的差异项。
type DiffEntry struct {
	Path  string `json:"path"`  // 点号分隔的字段路径，如 "messages.0.content" 或 "max_tokens"
	Raw   any    `json:"raw"`   // 转换前的值（lost/changed 时有值，added 时为 nil）
	After any    `json:"after"` // 转换后的值（added/changed 时有值，lost 时为 nil）
}

// DiffResult 是两份 JSON 的结构化差异报告。
type DiffResult struct {
	LostFields    []DiffEntry `json:"lost_fields"`    // raw 有、转换后没有的字段
	AddedFields   []DiffEntry `json:"added_fields"`   // 转换后新增的字段
	ChangedValues []DiffEntry `json:"changed_values"` // 同名字段值不同
}

// JSONDiff 对比两份 JSON（转换前 raw 与转换后 after），返回结构化差异。
// 纯函数：无 IO、无全局状态，相同输入永远相同输出。
// 只做 JSON 结构级对比，不做协议语义映射。
func JSONDiff(rawJSON, afterJSON []byte) (*DiffResult, error) {
	result := &DiffResult{
		LostFields:    []DiffEntry{},
		AddedFields:   []DiffEntry{},
		ChangedValues: []DiffEntry{},
	}

	var raw, after any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(afterJSON, &after); err != nil {
		return nil, err
	}

	diffValue("", raw, after, result)
	return result, nil
}

// diffValue 递归对比两个任意类型的值，将差异写入 result。
func diffValue(path string, raw, after any, result *DiffResult) {
	rawMap, rawIsMap := raw.(map[string]any)
	afterMap, afterIsMap := after.(map[string]any)

	if rawIsMap && afterIsMap {
		diffMap(path, rawMap, afterMap, result)
		return
	}

	rawSlice, rawIsSlice := raw.([]any)
	afterSlice, afterIsSlice := after.([]any)

	if rawIsSlice && afterIsSlice {
		diffSlice(path, rawSlice, afterSlice, result)
		return
	}

	// 类型不同或标量值不同
	if !jsonEqual(raw, after) {
		result.ChangedValues = append(result.ChangedValues, DiffEntry{
			Path:  path,
			Raw:   raw,
			After: after,
		})
	}
}

// diffMap 对比两个 map，分 lost / added / 嵌套递归。
// key 排序后遍历，保证输出顺序稳定。
func diffMap(path string, rawMap, afterMap map[string]any, result *DiffResult) {
	rawKeys := make([]string, 0, len(rawMap))
	for key := range rawMap {
		rawKeys = append(rawKeys, key)
	}
	sort.Strings(rawKeys)

	for _, key := range rawKeys {
		childPath := joinPath(path, key)
		rawVal := rawMap[key]
		afterVal, exists := afterMap[key]
		if !exists {
			result.LostFields = append(result.LostFields, DiffEntry{
				Path:  childPath,
				Raw:   rawVal,
				After: nil,
			})
			continue
		}
		diffValue(childPath, rawVal, afterVal, result)
	}

	afterKeys := make([]string, 0, len(afterMap))
	for key := range afterMap {
		afterKeys = append(afterKeys, key)
	}
	sort.Strings(afterKeys)

	for _, key := range afterKeys {
		if _, exists := rawMap[key]; !exists {
			result.AddedFields = append(result.AddedFields, DiffEntry{
				Path:  joinPath(path, key),
				Raw:   nil,
				After: afterMap[key],
			})
		}
	}
}

// diffSlice 对比两个 slice，按下标递归。
func diffSlice(path string, rawSlice, afterSlice []any, result *DiffResult) {
	maxLen := len(rawSlice)
	if len(afterSlice) > maxLen {
		maxLen = len(afterSlice)
	}

	for i := 0; i < maxLen; i++ {
		childPath := joinPath(path, indexString(i))
		if i >= len(rawSlice) {
			result.AddedFields = append(result.AddedFields, DiffEntry{
				Path:  childPath,
				Raw:   nil,
				After: afterSlice[i],
			})
			continue
		}
		if i >= len(afterSlice) {
			result.LostFields = append(result.LostFields, DiffEntry{
				Path:  childPath,
				Raw:   rawSlice[i],
				After: nil,
			})
			continue
		}
		diffValue(childPath, rawSlice[i], afterSlice[i], result)
	}
}

// joinPath 拼接字段路径。
func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// indexString 将 slice 下标转为字符串。
func indexString(i int) string {
	return strconv.Itoa(i)
}

// jsonEqual 判断两个 any（从 JSON 解析）是否语义相等。
// json.Unmarshal 把数字都解析为 float64，所以直接用 == 即可。
func jsonEqual(a, b any) bool {
	return a == b
}
